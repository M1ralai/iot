package mqtt

import (
	"errors"
	"fmt"
	"time"

	"github.com/M1ralai/iot/src/server/mqtt/ack"
	"github.com/M1ralai/iot/src/server/mqtt/heartbeat"
	"github.com/M1ralai/iot/src/server/mqtt/schema"
	"github.com/M1ralai/iot/src/server/mqtt/status"
	"github.com/M1ralai/iot/src/server/mqtt/telemetry"
	paho "github.com/eclipse/paho.mqtt.golang"
)

const (
	subscribeQoS     = 1
	subscribeTimeout = 5 * time.Second
	// subscribeFailed is the SUBACK return code for a refused subscription.
	subscribeFailed = 0x80
)

type route struct {
	topic   string
	handler paho.MessageHandler
}

// routes lists every incoming topic the backend must be subscribed to before
// it is considered ready.
func routes(schemas *schema.Schemas) []route {
	return []route{
		{telemetry.Topic, telemetry.Handle(schemas)},
		{heartbeat.Topic, heartbeat.Handle(schemas)},
		{ack.Topic, ack.Handle(schemas)},
		{status.Topic, status.Handle(schemas)},
	}
}

// subscribeAll registers every route and returns all failures joined. The
// subscriptions are idempotent, so it is safe to call again after a failure.
func subscribeAll(client paho.Client, routes []route) error {
	var errs []error
	for _, r := range routes {
		if err := subscribe(client, r); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func subscribe(client paho.Client, r route) error {
	token := client.Subscribe(r.topic, subscribeQoS, r.handler)
	if !token.WaitTimeout(subscribeTimeout) {
		return fmt.Errorf("subscribe %s: timed out after %s", r.topic, subscribeTimeout)
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("subscribe %s: %w", r.topic, err)
	}
	if st, ok := token.(*paho.SubscribeToken); ok && st.Result()[r.topic] == subscribeFailed {
		return fmt.Errorf("subscribe %s: refused by broker", r.topic)
	}
	return nil
}
