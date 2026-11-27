package start

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/M1ralai/iot/src/server/mqtt/schema"
	paho "github.com/eclipse/paho.mqtt.golang"
)

const (
	// Topic is the command topic shared by start, permit and stop.
	Topic = "irrigation/v1/gw-01/commands/node-01"
	kind  = "start"

	publishTimeout = 3 * time.Second
)

var errExpired = errors.New("command has already expired")

// Publish validates and sends the start command with QoS 1 and retain=false.
// The command is serialized, validated as serialized against its schema, and
// published only if every check passes.
//
// A nil error means the broker acknowledged the message (PUBACK). It does not
// mean the gateway or node received or applied the command; that is reported
// only by an ack message.
//
// If the returned error wraps context.DeadlineExceeded or context.Canceled the
// delivery state is unknown: the message may still reach the broker. The
// command must not be blindly retried with a new identity.
func Publish(ctx context.Context, client paho.Client, schemas *schema.Schemas, payload Payload) error {
	payload.Kind = kind
	payload.SchemaVersion = schemas.Version()
	payload.IssuedAt = payload.IssuedAt.UTC()
	payload.ExpiresAt = payload.ExpiresAt.UTC()

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%s: encode payload: %w", kind, err)
	}
	if _, err := schemas.Validate(kind, data); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}
	if err := schemas.CheckCommandWindow(payload.IssuedAt, payload.ExpiresAt); err != nil {
		return fmt.Errorf("%s: invalid command: %w", kind, err)
	}
	if !time.Now().Before(payload.ExpiresAt) {
		return fmt.Errorf("%s: invalid command: %w", kind, errExpired)
	}
	ctx, cancel := context.WithTimeout(ctx, publishTimeout)
	defer cancel()

	token := client.Publish(Topic, 1, false, data)
	select {
	case <-token.Done():
		if err := token.Error(); err != nil {
			return fmt.Errorf("%s: publish: %w", kind, err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%s: publish not confirmed, delivery state unknown: %w", kind, ctx.Err())
	}
}
