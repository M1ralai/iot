// Package schema owns validation of MQTT payloads against the embedded JSON
// Schema snapshots of the shared protocol definitions.
//
// The three embedded files are local copies of shared/mqtt/message.schema.json,
// shared/espnow/frame.schema.json and shared/rules/safety-profile.json. The
// first two are JSON Schemas; safety-profile.json is protocol configuration
// and is only read for the limits this package enforces.
package schema

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed message.schema.json frame.schema.json safety-profile.json
var files embed.FS

const (
	messageID = "urn:irrigation:shared:mqtt:v1"
	frameID   = "urn:irrigation:shared:frame:v2"

	// maxExponent bounds number exponents before they reach arbitrary
	// precision arithmetic. Every numeric field in the protocol is far smaller.
	maxExponent = 400
	// maxErrors limits how many schema violations appear in an error message.
	maxErrors = 3
)

// kinds are the seven message kinds that must each have a schema branch.
var kinds = []string{"telemetry", "heartbeat", "ack", "start", "permit", "stop", "status"}

// profile is the subset of safety-profile.json used by this package.
type profile struct {
	MQTTSchemaVersion    int `json:"mqtt_schema_version"`
	CommandDispatchTTLMS int `json:"command_dispatch_ttl_ms"`
	MaxMQTTPayloadBytes  int `json:"max_mqtt_payload_bytes"`
}

// Schemas holds one compiled validator per message kind. It is immutable after
// Load and safe for concurrent use.
type Schemas struct {
	byKind     map[string]*jsonschema.Schema
	version    int
	commandTTL time.Duration
	maxPayload int
}

// Load compiles every message schema from the embedded files. Any failure to
// parse, resolve references or map a message kind to its schema is an error;
// there is no fallback.
func Load() (*Schemas, error) {
	var p profile
	if err := readInto("safety-profile.json", &p); err != nil {
		return nil, err
	}
	if p.MQTTSchemaVersion < 1 || p.CommandDispatchTTLMS < 1 || p.MaxMQTTPayloadBytes < 1 {
		return nil, fmt.Errorf("schema: safety-profile.json is missing mqtt_schema_version, command_dispatch_ttl_ms or max_mqtt_payload_bytes")
	}

	message, err := readDocument("message.schema.json", messageID)
	if err != nil {
		return nil, err
	}
	frame, err := readDocument("frame.schema.json", frameID)
	if err != nil {
		return nil, err
	}

	branches, err := branchIndexes(message)
	if err != nil {
		return nil, err
	}

	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat() // date-time must be asserted, not merely annotated
	if err := c.AddResource(messageID, message); err != nil {
		return nil, fmt.Errorf("schema: add message schema: %w", err)
	}
	if err := c.AddResource(frameID, frame); err != nil {
		return nil, fmt.Errorf("schema: add frame schema: %w", err)
	}

	s := &Schemas{
		byKind:     make(map[string]*jsonschema.Schema, len(kinds)),
		version:    p.MQTTSchemaVersion,
		commandTTL: time.Duration(p.CommandDispatchTTLMS) * time.Millisecond,
		maxPayload: p.MaxMQTTPayloadBytes,
	}
	for _, kind := range kinds {
		sch, err := c.Compile(fmt.Sprintf("%s#/oneOf/%d", messageID, branches[kind]))
		if err != nil {
			return nil, fmt.Errorf("schema: compile %s: %w", kind, err)
		}
		s.byKind[kind] = sch
	}

	return s, nil
}

// Version is the MQTT schema_version outgoing messages must carry.
func (s *Schemas) Version() int { return s.version }

// CheckCommandWindow enforces 0 < expires-issued <= command_dispatch_ttl_ms.
// The schema only requires both values to be date-times.
func (s *Schemas) CheckCommandWindow(issued, expires time.Time) error {
	window := expires.Sub(issued)
	if window <= 0 || window > s.commandTTL {
		return fmt.Errorf("expires_at-issued_at must be in (0, %s], got %s", s.commandTTL, window)
	}
	return nil
}

// Validate checks data, the complete JSON text of one message of the given
// kind, and returns it in canonical form ready for typed decoding.
//
// It enforces the payload size limit, UTF-8, strict JSON (a single document,
// no duplicate property names at any depth) and the compiled schema. Numbers
// with an integral value are rewritten without fraction or exponent ("1.0"
// and "1e1" become "1" and "10"), because the schema treats them as integers
// while Go's integer decoding does not.
func (s *Schemas) Validate(kind string, data []byte) ([]byte, error) {
	sch, ok := s.byKind[kind]
	if !ok {
		return nil, fmt.Errorf("schema: unknown message kind %q", kind)
	}
	if len(data) > s.maxPayload {
		return nil, fmt.Errorf("%s: payload is %d bytes, limit is %d", kind, len(data), s.maxPayload)
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("%s: payload is not valid UTF-8", kind)
	}

	doc, err := parseStrict(data)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid JSON: %w", kind, err)
	}
	if err := sch.Validate(doc); err != nil {
		return nil, &invalidError{kind: kind, detail: summarize(err), err: err}
	}

	canonical, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("%s: canonicalize validated payload: %w", kind, err)
	}
	return canonical, nil
}

type invalidError struct {
	kind   string
	detail string
	err    error
}

func (e *invalidError) Error() string { return e.kind + ": schema violation: " + e.detail }
func (e *invalidError) Unwrap() error { return e.err }

// summarize reduces a validation failure to a few "location: problem" pairs,
// never including the offending values' surrounding payload.
func summarize(err error) string {
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return err.Error()
	}

	var parts []string
	collectLeaves(*ve.DetailedOutput(), &parts)
	if len(parts) == 0 {
		return "does not match schema"
	}
	return strings.Join(parts, "; ")
}

// collectLeaves gathers the innermost errors of a detailed output tree, which
// carry the actual field and constraint; enclosing units only say "allOf
// failed" or "validation failed".
func collectLeaves(u jsonschema.OutputUnit, parts *[]string) {
	if len(*parts) >= maxErrors {
		return
	}
	if len(u.Errors) == 0 {
		if u.Error != nil {
			loc := u.InstanceLocation
			if loc == "" {
				loc = "/"
			}
			*parts = append(*parts, loc+": "+u.Error.String())
		}
		return
	}
	for _, c := range u.Errors {
		collectLeaves(c, parts)
	}
}

func readDocument(name, wantID string) (any, error) {
	b, err := files.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("schema: read %s: %w", name, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("schema: parse %s: %w", name, err)
	}
	if obj, _ := doc.(map[string]any); obj["$id"] != wantID {
		return nil, fmt.Errorf("schema: %s must declare $id %q", name, wantID)
	}
	return doc, nil
}

func readInto(name string, v any) error {
	b, err := files.ReadFile(name)
	if err != nil {
		return fmt.Errorf("schema: read %s: %w", name, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("schema: parse %s: %w", name, err)
	}
	return nil
}

// branchIndexes maps each message kind to its index in the message schema's
// top-level oneOf, by reading each branch's kind constant.
func branchIndexes(message any) (map[string]int, error) {
	obj, _ := message.(map[string]any)
	oneOf, _ := obj["oneOf"].([]any)

	indexes := make(map[string]int, len(oneOf))
	for i, branch := range oneOf {
		b, _ := branch.(map[string]any)
		props, _ := b["properties"].(map[string]any)
		kind, _ := props["kind"].(map[string]any)
		name, ok := kind["const"].(string)
		if !ok {
			return nil, fmt.Errorf("schema: oneOf branch %d of message schema has no kind constant", i)
		}
		if _, dup := indexes[name]; dup {
			return nil, fmt.Errorf("schema: message kind %q appears twice in message schema", name)
		}
		indexes[name] = i
	}
	for _, kind := range kinds {
		if _, ok := indexes[kind]; !ok {
			return nil, fmt.Errorf("schema: message schema has no branch for kind %q", kind)
		}
	}
	return indexes, nil
}

// parseStrict decodes exactly one JSON document into the value types the
// validator expects (map[string]any, []any, string, bool, nil, json.Number),
// rejecting duplicate property names at every depth and trailing content.
func parseStrict(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	doc, err := parseValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("trailing content after JSON document")
		}
		return nil, fmt.Errorf("trailing content after JSON document: %w", err)
	}
	return doc, nil
}

func parseValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err == io.EOF {
		return nil, io.ErrUnexpectedEOF
	}
	if err != nil {
		return nil, err
	}

	switch t := tok.(type) {
	case json.Delim:
		if t == '{' {
			return parseObject(dec)
		}
		return parseArray(dec)
	case json.Number:
		return normalizeNumber(t)
	default: // string, bool, nil
		return t, nil
	}
}

func parseObject(dec *json.Decoder) (any, error) {
	obj := make(map[string]any)
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected object key %v", tok)
		}
		if _, dup := obj[key]; dup {
			return nil, fmt.Errorf("duplicate property %q", key)
		}
		if obj[key], err = parseValue(dec); err != nil {
			return nil, err
		}
	}
	if _, err := dec.Token(); err != nil { // closing brace
		return nil, err
	}
	return obj, nil
}

func parseArray(dec *json.Decoder) (any, error) {
	arr := []any{}
	for dec.More() {
		v, err := parseValue(dec)
		if err != nil {
			return nil, err
		}
		arr = append(arr, v)
	}
	if _, err := dec.Token(); err != nil { // closing bracket
		return nil, err
	}
	return arr, nil
}

// normalizeNumber rewrites integral numbers as plain integers and leaves
// other numbers byte-for-byte untouched, so decimal precision (for example
// multipleOf 0.01) is not lost to floating point.
func normalizeNumber(n json.Number) (json.Number, error) {
	s := n.String()
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		exp, err := strconv.Atoi(s[i+1:])
		if err != nil || exp > maxExponent || exp < -maxExponent {
			return "", fmt.Errorf("number exponent out of supported range")
		}
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return "", fmt.Errorf("invalid number")
	}
	if r.IsInt() {
		return json.Number(r.Num().String()), nil
	}
	return n, nil
}
