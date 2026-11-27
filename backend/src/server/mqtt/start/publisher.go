package start

import (
	"encoding/json"
	"fmt"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

func Publish(client paho.Client, payload Payload) error {
	payload.Kind = "stop"
	payload.SchemaVersion = 1

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	topic := "irrigation/v1/gw-01/commands/node-01"

	token := client.Publish(topic, 1, false, data)

	if !token.WaitTimeout(3 * time.Second) {
		return fmt.Errorf("stop: publish timeout")
	}

	return token.Error()
}
