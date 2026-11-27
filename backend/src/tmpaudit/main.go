// TEMPORARY audit harness - deleted after the audit.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/M1ralai/iot/src/server/mqtt"
	"github.com/M1ralai/iot/src/server/mqtt/permit"
	"github.com/M1ralai/iot/src/server/mqtt/schema"
	"github.com/M1ralai/iot/src/server/mqtt/start"
	"github.com/M1ralai/iot/src/server/mqtt/stop"
)

var seq int

func mid() string { seq++; return fmt.Sprintf("%016x", 0xa0000000+seq) }

type env struct {
	m   *mqtt.MQTT
	ctx context.Context
}

func report(kind, name string, wantErr bool, err error, id string) {
	got := "SENT-OK"
	if err != nil {
		got = "ERR: " + strings.ReplaceAll(err.Error(), "\n", " ")
	}
	res := "PASS"
	if wantErr != (err != nil) {
		res = "FAIL"
	}
	fmt.Printf("RESULT|%s|%s|%s|wantErr=%v|%s|mid=%s\n", res, kind, name, wantErr, got, id)
}

func now() time.Time { return time.Now().UTC().Truncate(time.Millisecond) }

func startBase(ttl time.Duration) start.Payload {
	n := now()
	return start.Payload{GatewayID: "gw-01", GatewayBootID: "0a0b0c0d0e0f1011", GatewaySessionID: "1112131415161718", MessageID: mid(), NodeID: "node-01",
		BackendEpoch: "2122232425262728", CommandID: "2233445566778899", RunID: "1122334455667788", TargetBootID: "0102030405060708",
		IssuedAt: n, ExpiresAt: n.Add(ttl), Payload: start.Command{ArmNonce: "33445566778899aa", DurationMS: 15000, PermitBudgetMS: 3000}}
}
func permitBase(ttl time.Duration) permit.Payload {
	s := startBase(ttl)
	return permit.Payload{GatewayID: s.GatewayID, GatewayBootID: s.GatewayBootID, GatewaySessionID: s.GatewaySessionID, MessageID: mid(), NodeID: s.NodeID,
		BackendEpoch: s.BackendEpoch, CommandID: s.CommandID, RunID: s.RunID, TargetBootID: s.TargetBootID, IssuedAt: s.IssuedAt, ExpiresAt: s.ExpiresAt,
		Payload: permit.Command{ArmNonce: "33445566778899aa", PermitBudgetMS: 3000}}
}
func stopBase(ttl time.Duration) stop.Payload {
	s := startBase(ttl)
	return stop.Payload{GatewayID: s.GatewayID, GatewayBootID: s.GatewayBootID, GatewaySessionID: s.GatewaySessionID, MessageID: mid(), NodeID: s.NodeID,
		BackendEpoch: s.BackendEpoch, CommandID: s.CommandID, RunID: s.RunID, TargetBootID: s.TargetBootID, IssuedAt: s.IssuedAt, ExpiresAt: s.ExpiresAt}
}

var badIDs = map[string]string{"empty": "", "short": "0a0b0c0d0e0f101", "long": "0a0b0c0d0e0f10111", "upper": "0A0B0C0D0E0F1011", "nonhex": "zz0b0c0d0e0f1011", "zero": "0000000000000000", "newline": "0a0b0c0d0e0f1011\n"}

func main() {
	broker := os.Getenv("BROKER")
	ttl := 5 * time.Second
	schemas, err := schema.Load()
	if err != nil {
		fmt.Println("LOAD", err)
		os.Exit(1)
	}
	m := mqtt.New(mqtt.Config{Broker: broker, ClientID: "audit-go-harness-" + fmt.Sprint(os.Getpid())}, schemas)
	ctx := context.Background()

	// disconnected before Connect
	id := mid()
	p := startBase(ttl)
	p.MessageID = id
	report("start", "publish before Connect (never connected)", true, m.PublishStart(ctx, p), id)
	fmt.Println("INFO status before connect:", m.Status(), "ready:", m.Ready())

	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	if err := m.Connect(cctx); err != nil {
		fmt.Println("CONNECT FAILED", err)
		os.Exit(1)
	}
	cancel()
	fmt.Println("INFO status after connect:", m.Status(), "ready:", m.Ready())

	// ---- START
	st := func(name string, wantErr bool, mod func(p *start.Payload)) {
		p := startBase(ttl)
		mod(&p)
		report("start", name, wantErr, m.PublishStart(ctx, p), p.MessageID)
	}
	st("valid", false, func(p *start.Payload) {})
	st("duration=1 (min)", false, func(p *start.Payload) { p.Payload.DurationMS = 1 })
	st("duration=25000 (max)", false, func(p *start.Payload) { p.Payload.DurationMS = 25000 })
	st("duration=0", true, func(p *start.Payload) { p.Payload.DurationMS = 0 })
	st("duration=25001", true, func(p *start.Payload) { p.Payload.DurationMS = 25001 })
	for _, b := range []uint32{0, 2999, 3001, 4294967295} {
		b := b
		st(fmt.Sprintf("permit_budget=%d", b), true, func(p *start.Payload) { p.Payload.PermitBudgetMS = b })
	}
	for n, v := range badIDs {
		v := v
		st("arm_nonce "+n, true, func(p *start.Payload) { p.Payload.ArmNonce = v })
		st("gateway_boot_id "+n, true, func(p *start.Payload) { p.GatewayBootID = v })
		st("gateway_session_id "+n, true, func(p *start.Payload) { p.GatewaySessionID = v })
		st("backend_epoch "+n, true, func(p *start.Payload) { p.BackendEpoch = v })
		st("command_id "+n, true, func(p *start.Payload) { p.CommandID = v })
		st("run_id "+n, true, func(p *start.Payload) { p.RunID = v })
		st("target_boot_id "+n, true, func(p *start.Payload) { p.TargetBootID = v })
		if n != "empty" && n != "zero" || true {
			st("message_id "+n, true, func(p *start.Payload) { p.MessageID = v })
		}
	}
	st("gateway_id gw-02", true, func(p *start.Payload) { p.GatewayID = "gw-02" })
	st("gateway_id empty", true, func(p *start.Payload) { p.GatewayID = "" })
	st("node_id node-02", true, func(p *start.Payload) { p.NodeID = "node-02" })
	st("node_id empty", true, func(p *start.Payload) { p.NodeID = "" })
	st("caller-supplied kind/schema_version are overridden", false, func(p *start.Payload) { p.Kind = "stop"; p.SchemaVersion = 99 })
	st("window=TTL (5000ms max valid)", false, func(p *start.Payload) { p.ExpiresAt = p.IssuedAt.Add(5000 * time.Millisecond) })
	st("window=1ms (min positive)", false, func(p *start.Payload) { p.ExpiresAt = p.IssuedAt.Add(time.Millisecond) })
	st("window=TTL+1ms", true, func(p *start.Payload) { p.ExpiresAt = p.IssuedAt.Add(5001 * time.Millisecond) })
	st("window=0", true, func(p *start.Payload) { p.ExpiresAt = p.IssuedAt })
	st("window=-1ms", true, func(p *start.Payload) { p.ExpiresAt = p.IssuedAt.Add(-time.Millisecond) })
	st("window=-5s", true, func(p *start.Payload) { p.ExpiresAt = p.IssuedAt.Add(-5 * time.Second) })
	st("already expired, valid 4s window", true, func(p *start.Payload) { p.IssuedAt = now().Add(-10 * time.Second); p.ExpiresAt = p.IssuedAt.Add(4 * time.Second) })
	st("expires exactly now (1ms past)", true, func(p *start.Payload) { p.IssuedAt = now().Add(-2 * time.Second); p.ExpiresAt = now().Add(-time.Millisecond) })
	st("issued in the future (+1h), valid window", false, func(p *start.Payload) { p.IssuedAt = now().Add(time.Hour); p.ExpiresAt = p.IssuedAt.Add(time.Second) })
	st("zero IssuedAt", true, func(p *start.Payload) { p.IssuedAt = time.Time{} })
	st("zero ExpiresAt", true, func(p *start.Payload) { p.ExpiresAt = time.Time{} })
	st("year 10000 (unrepresentable)", true, func(p *start.Payload) { p.ExpiresAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC) })
	st("non-UTC zone converted to UTC", false, func(p *start.Payload) {
		z := time.FixedZone("x", 3*3600)
		p.IssuedAt = p.IssuedAt.In(z)
		p.ExpiresAt = p.ExpiresAt.In(z)
	})
	st("nanosecond precision timestamps", false, func(p *start.Payload) { p.IssuedAt = time.Now(); p.ExpiresAt = p.IssuedAt.Add(2*time.Second + 123456789) })

	// ---- PERMIT
	pm := func(name string, wantErr bool, mod func(p *permit.Payload)) {
		p := permitBase(ttl)
		mod(&p)
		report("permit", name, wantErr, m.PublishPermit(ctx, p), p.MessageID)
	}
	pm("valid", false, func(p *permit.Payload) {})
	for _, b := range []uint32{0, 2999, 3001} {
		b := b
		pm(fmt.Sprintf("permit_budget=%d", b), true, func(p *permit.Payload) { p.Payload.PermitBudgetMS = b })
	}
	for n, v := range badIDs {
		v := v
		pm("arm_nonce "+n, true, func(p *permit.Payload) { p.Payload.ArmNonce = v })
		pm("command_id "+n, true, func(p *permit.Payload) { p.CommandID = v })
		pm("run_id "+n, true, func(p *permit.Payload) { p.RunID = v })
		pm("target_boot_id "+n, true, func(p *permit.Payload) { p.TargetBootID = v })
		pm("backend_epoch "+n, true, func(p *permit.Payload) { p.BackendEpoch = v })
		pm("gateway_boot_id "+n, true, func(p *permit.Payload) { p.GatewayBootID = v })
		pm("gateway_session_id "+n, true, func(p *permit.Payload) { p.GatewaySessionID = v })
		pm("message_id "+n, true, func(p *permit.Payload) { p.MessageID = v })
	}
	pm("gateway_id gw-02", true, func(p *permit.Payload) { p.GatewayID = "gw-02" })
	pm("node_id node-02", true, func(p *permit.Payload) { p.NodeID = "node-02" })
	pm("kind overridden", false, func(p *permit.Payload) { p.Kind = "start"; p.SchemaVersion = 7 })
	pm("window=5000ms", false, func(p *permit.Payload) { p.ExpiresAt = p.IssuedAt.Add(5000 * time.Millisecond) })
	pm("window=1ms", false, func(p *permit.Payload) { p.ExpiresAt = p.IssuedAt.Add(time.Millisecond) })
	pm("window=5001ms", true, func(p *permit.Payload) { p.ExpiresAt = p.IssuedAt.Add(5001 * time.Millisecond) })
	pm("window=0", true, func(p *permit.Payload) { p.ExpiresAt = p.IssuedAt })
	pm("window negative", true, func(p *permit.Payload) { p.ExpiresAt = p.IssuedAt.Add(-time.Second) })
	pm("already expired, valid 4s window", true, func(p *permit.Payload) { p.IssuedAt = now().Add(-10 * time.Second); p.ExpiresAt = p.IssuedAt.Add(4 * time.Second) })
	pm("zero IssuedAt", true, func(p *permit.Payload) { p.IssuedAt = time.Time{} })

	// ---- STOP
	sp := func(name string, wantErr bool, mod func(p *stop.Payload)) {
		p := stopBase(ttl)
		mod(&p)
		report("stop", name, wantErr, m.PublishStop(ctx, p), p.MessageID)
	}
	sp("valid (empty payload object)", false, func(p *stop.Payload) {})
	for n, v := range badIDs {
		v := v
		sp("command_id "+n, true, func(p *stop.Payload) { p.CommandID = v })
		sp("run_id "+n, true, func(p *stop.Payload) { p.RunID = v })
		sp("target_boot_id "+n, true, func(p *stop.Payload) { p.TargetBootID = v })
		sp("backend_epoch "+n, true, func(p *stop.Payload) { p.BackendEpoch = v })
		sp("gateway_boot_id "+n, true, func(p *stop.Payload) { p.GatewayBootID = v })
		sp("gateway_session_id "+n, true, func(p *stop.Payload) { p.GatewaySessionID = v })
		sp("message_id "+n, true, func(p *stop.Payload) { p.MessageID = v })
	}
	sp("gateway_id gw-02", true, func(p *stop.Payload) { p.GatewayID = "gw-02" })
	sp("node_id node-02", true, func(p *stop.Payload) { p.NodeID = "node-02" })
	sp("kind overridden", false, func(p *stop.Payload) { p.Kind = "start"; p.SchemaVersion = 7 })
	sp("window=5000ms", false, func(p *stop.Payload) { p.ExpiresAt = p.IssuedAt.Add(5000 * time.Millisecond) })
	sp("window=1ms", false, func(p *stop.Payload) { p.ExpiresAt = p.IssuedAt.Add(time.Millisecond) })
	sp("window=5001ms", true, func(p *stop.Payload) { p.ExpiresAt = p.IssuedAt.Add(5001 * time.Millisecond) })
	sp("window=0", true, func(p *stop.Payload) { p.ExpiresAt = p.IssuedAt })
	sp("window negative", true, func(p *stop.Payload) { p.ExpiresAt = p.IssuedAt.Add(-time.Second) })
	sp("already expired, valid 4s window (documented exception)", false, func(p *stop.Payload) { p.IssuedAt = now().Add(-10 * time.Second); p.ExpiresAt = p.IssuedAt.Add(4 * time.Second) })
	sp("already expired AND invalid window", true, func(p *stop.Payload) { p.IssuedAt = now().Add(-20 * time.Second); p.ExpiresAt = p.IssuedAt.Add(8 * time.Second) })
	sp("zero IssuedAt", true, func(p *stop.Payload) { p.IssuedAt = time.Time{} })

	// ---- context behaviour
	cc, ccancel := context.WithCancel(ctx)
	ccancel()
	p1 := startBase(ttl)
	e1 := m.PublishStart(cc, p1)
	report("start", "context already cancelled (must not publish)", true, e1, p1.MessageID)
	if e1 != nil {
		fmt.Println("INFO cancelled ctx err is context.Canceled:", errors.Is(e1, context.Canceled))
	}
	ec, ecancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
	defer ecancel()
	p2 := permitBase(ttl)
	report("permit", "context deadline already passed (must not publish)", true, m.PublishPermit(ec, p2), p2.MessageID)
	p3 := stopBase(ttl)
	report("stop", "context already cancelled (STOP, must not publish)", true, m.PublishStop(cc, p3), p3.MessageID)

	if os.Getenv("PHASE") == "timeout" {
		fmt.Println("INFO waiting for broker to be paused; sleeping 6s")
		time.Sleep(6 * time.Second)
		p := startBase(ttl)
		t0 := time.Now()
		err := m.PublishStart(ctx, p)
		fmt.Printf("TIMEOUT start err=%v elapsed=%s isDeadline=%v mid=%s\n", err, time.Since(t0).Round(time.Millisecond), errors.Is(err, context.DeadlineExceeded), p.MessageID)
		pc, pcancel := context.WithTimeout(ctx, 500*time.Millisecond)
		p = startBase(ttl)
		t0 = time.Now()
		err = m.PublishStart(pc, p)
		pcancel()
		fmt.Printf("TIMEOUT start(ctx 500ms) err=%v elapsed=%s isDeadline=%v mid=%s\n", err, time.Since(t0).Round(time.Millisecond), errors.Is(err, context.DeadlineExceeded), p.MessageID)
		ps := permitBase(ttl)
		fmt.Printf("TIMEOUT permit err=%v mid=%s\n", m.PublishPermit(ctx, ps), ps.MessageID)
		fmt.Println("INFO status while broker paused:", m.Status())
		time.Sleep(8 * time.Second)
	}

	m.Close()
	p = startBase(ttl)
	report("start", "publish after Close", true, m.PublishStart(ctx, p), p.MessageID)
	pp := permitBase(ttl)
	report("permit", "publish after Close", true, m.PublishPermit(ctx, pp), pp.MessageID)
	ps := stopBase(ttl)
	report("stop", "publish after Close", true, m.PublishStop(ctx, ps), ps.MessageID)
	fmt.Println("DONE")
}
