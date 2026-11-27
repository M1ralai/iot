package mqtt

import (
	"log"

	"github.com/M1ralai/iot/src/server/mqtt/ack"
	"github.com/M1ralai/iot/src/server/mqtt/heartbeat"
	"github.com/M1ralai/iot/src/server/mqtt/status"
	"github.com/M1ralai/iot/src/server/mqtt/telemetry"
	paho "github.com/eclipse/paho.mqtt.golang"
)

func RegisterRoutes(client paho.Client) {
	routes := map[string]paho.MessageHandler{
		"irrigation/v1/gw-01/telemetry": telemetry.Handle,
		"irrigation/v1/gw-01/heartbeat": heartbeat.Handle,
		"irrigation/v1/gw-01/acks":      ack.Handle,
		"irrigation/v1/gw-01/status":    status.Handle,
	}

	for topic, handler := range routes {
		token := client.Subscribe(topic, 1, handler)
		token.Wait()

		if err := token.Error(); err != nil {
			log.Printf("mqtt: subscribe %s: %v", topic, err)
			continue
		}

		log.Printf("mqtt: subscribed %s", topic)
	}
}
