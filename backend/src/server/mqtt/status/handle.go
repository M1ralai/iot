package status

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
	Topic = "irrigation/v1/gw-01/status"
	kind  = "status"
)

// Retained replays are accepted: the broker replays the last status on
// subscribe, and the gateway LWT is published retained.
//
// Handle returns the handler for incoming status messages. The handler is
// stateless and safe for concurrent use; it never blocks on the MQTT client.
// A rejected message is logged and otherwise ignored.
func Handle(schemas *schema.Schemas) paho.MessageHandler {
	return func(_ paho.Client, message paho.Message) {
		payload, err := decode(schemas, message)
		if err != nil {
			log.Printf("status: rejected message on %s: %v", message.Topic(), err)
			return
		}

		log.Printf("status: gateway %s is %s message_id=%s gateway_session_id=%s retained=%t",
			payload.GatewayID, payload.State, payload.MessageID, payload.GatewaySessionID, message.Retained())
	}
}

func decode(schemas *schema.Schemas, message paho.Message) (Payload, error) {
	var payload Payload

	if message.Topic() != Topic {
		return payload, errors.New("message delivered on unexpected topic")
	}
	data, err := schemas.Validate(kind, message.Payload())
	if err != nil {
		return payload, err
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return payload, fmt.Errorf("status: decode validated payload: %w", err)
	}

	return payload, nil
}
