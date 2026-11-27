package telemetry

import (
	"encoding/json"
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func Handle(client mqtt.Client, message mqtt.Message) {
	var payload Payload

	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Printf("telemetry: decode error: %v", err)
		return
	}

	log.Printf("telemetry: %+v", payload)
}
