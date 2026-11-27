package telemetry

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/M1ralai/iot/src/server/mqtt/schema"
	paho "github.com/eclipse/paho.mqtt.golang"
)

const (
	// Topic is the gateway-to-backend topic this package consumes.
	Topic = "irrigation/v1/gw-01/telemetry"
	kind  = "telemetry"
)

// Handle returns the handler for incoming telemetry messages. The handler is
// stateless and safe for concurrent use; it never blocks on the MQTT client.
// A rejected message is logged and otherwise ignored.
func Handle(schemas *schema.Schemas) paho.MessageHandler {
	return func(_ paho.Client, message paho.Message) {
		payload, err := decode(schemas, message)
		if err != nil {
			log.Printf("telemetry: rejected message on %s: %v", message.Topic(), err)
			return
		}

		log.Printf("%s: received message_id=%s gateway_session_id=%s node_id=%s historical=%t",
			kind, payload.MessageID, payload.GatewaySessionID, payload.NodeID, payload.Historical)
	}
}

func decode(schemas *schema.Schemas, message paho.Message) (Payload, error) {
	var payload Payload

	if message.Topic() != Topic {
		return payload, errors.New("message delivered on unexpected topic")
	}
	// Only a message replayed from the broker's retained store is flagged as
	// retained; MQTT 3.1.1 clears the flag on live delivery, so a false value
	// does not prove the message was published without retain. A replay is
	// stale by definition and must not be treated as a live observation.
	if message.Retained() {
		return payload, errors.New("retained replay is stale and not accepted")
	}
	data, err := schemas.Validate(kind, message.Payload())
	if err != nil {
		return payload, err
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return payload, fmt.Errorf("telemetry: decode validated payload: %w", err)
	}

	return payload, nil
}
