package mqtt

import (
	"time"

	"github.com/M1ralai/iot/src/server/mqtt/permit"
	"github.com/M1ralai/iot/src/server/mqtt/start"
	"github.com/M1ralai/iot/src/server/mqtt/stop"
	paho "github.com/eclipse/paho.mqtt.golang"
)

type MQTT struct {
	client paho.Client
}

func New(broker, clientID string) *MQTT {
	opts := paho.NewClientOptions()

	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetAutoReconnect(true)
	opts.SetKeepAlive(30 * time.Second)
	opts.SetConnectTimeout(5 * time.Second)
	opts.SetOnConnectHandler(RegisterRoutes)

	return &MQTT{
		client: paho.NewClient(opts),
	}
}

func (m *MQTT) Connect() error {
	token := m.client.Connect()
	token.Wait()

	return token.Error()
}

func (m *MQTT) Close() {
	m.client.Disconnect(250)
}

func (m *MQTT) IsConnected() bool {
	return m.client.IsConnectionOpen()
}

func (m *MQTT) PublishStart(payload start.Payload) error {
	return start.Publish(m.client, payload)
}

func (m *MQTT) PublishPermit(payload permit.Payload) error {
	return permit.Publish(m.client, payload)
}

func (m *MQTT) PublishStop(payload stop.Payload) error {
	return stop.Publish(m.client, payload)
}
