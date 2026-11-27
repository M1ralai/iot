// Package mqtt owns the single Paho client used by the backend: its
// connection, subscriptions and the publishing API for irrigation commands.
package mqtt

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/M1ralai/iot/src/server/mqtt/permit"
	"github.com/M1ralai/iot/src/server/mqtt/schema"
	"github.com/M1ralai/iot/src/server/mqtt/start"
	"github.com/M1ralai/iot/src/server/mqtt/stop"
	paho "github.com/eclipse/paho.mqtt.golang"
)

const (
	connectTimeout    = 5 * time.Second
	readyTimeout      = 15 * time.Second
	disconnectQuiesce = 250 // milliseconds
	maxRetryBackoff   = 30 * time.Second
)

var (
	// ErrNotConnected is returned when a command is published while the broker
	// connection is down. Commands are never queued for later delivery.
	ErrNotConnected = errors.New("mqtt: not connected to broker")
	// ErrClosed is returned when the component is used after Close.
	ErrClosed = errors.New("mqtt: closed")
)

// Config describes the broker connection. Password is never logged.
type Config struct {
	Broker   string
	ClientID string
	Username string
	Password string
}

// Status describes the MQTT component only. Whether the gateway is online and
// whether the node is reachable are application-level facts carried by status
// and heartbeat messages; they are not implied by either field.
type Status struct {
	// Connected means a broker connection is currently open.
	Connected bool
	// Subscribed means every required subscription is registered on the
	// current connection.
	Subscribed bool
}

// Ready reports whether the component can receive and send messages.
func (s Status) Ready() bool { return s.Connected && s.Subscribed }

type phase int

const (
	phaseNew phase = iota
	phaseStarted
	phaseClosed
)

type MQTT struct {
	cfg    Config
	client paho.Client

	schemas *schema.Schemas
	routes  []route

	done      chan struct{} // closed by Close; stops subscription retries
	ready     chan struct{} // closed once the first full subscription succeeds
	readyOnce sync.Once
	wg        sync.WaitGroup // subscription goroutines

	mu         sync.Mutex // guards phase, generation and the subscribed flag
	phase      phase
	generation uint64 // incremented on every connect and connection loss
	subscribed atomic.Bool
}

// New builds the component from already compiled schemas. It performs no
// network I/O.
func New(cfg Config, schemas *schema.Schemas) *MQTT {
	m := &MQTT{
		cfg:     cfg,
		schemas: schemas,
		routes:  routes(schemas),
		done:    make(chan struct{}),
		ready:   make(chan struct{}),
	}

	opts := paho.NewClientOptions()
	opts.AddBroker(cfg.Broker)
	opts.SetClientID(cfg.ClientID)
	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
		opts.SetPassword(cfg.Password)
	}
	opts.SetCleanSession(true) // required by shared/mqtt/README.md
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(maxRetryBackoff)
	opts.SetKeepAlive(30 * time.Second)
	opts.SetConnectTimeout(connectTimeout)
	opts.SetOnConnectHandler(m.onConnect)
	opts.SetConnectionLostHandler(m.onConnectionLost)
	opts.SetReconnectingHandler(func(paho.Client, *paho.ClientOptions) {
		log.Printf("mqtt: reconnecting to %s", m.broker())
	})
	m.client = paho.NewClient(opts)

	return m
}

// Connect opens the broker connection and returns once every required
// subscription is registered, or fails. On failure the component is closed.
// After a successful Connect the client reconnects and resubscribes on its own.
func (m *MQTT) Connect(ctx context.Context) error {
	m.mu.Lock()
	if m.phase != phaseNew {
		m.mu.Unlock()
		return errors.New("mqtt: Connect called more than once")
	}
	m.phase = phaseStarted
	m.mu.Unlock()

	log.Printf("mqtt: connecting to %s as %s", m.broker(), m.cfg.ClientID)

	if err := m.connect(ctx); err != nil {
		m.Close()
		return err
	}
	return nil
}

func (m *MQTT) connect(ctx context.Context) error {
	token := m.client.Connect()
	select {
	case <-token.Done():
		if err := token.Error(); err != nil {
			return fmt.Errorf("mqtt: connect to %s: %w", m.broker(), err)
		}
	case <-ctx.Done():
		return fmt.Errorf("mqtt: connect to %s: %w", m.broker(), ctx.Err())
	}

	timer := time.NewTimer(readyTimeout)
	defer timer.Stop()
	select {
	case <-m.ready:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("mqtt: waiting for subscriptions: %w", ctx.Err())
	case <-timer.C:
		return fmt.Errorf("mqtt: subscriptions not registered within %s", readyTimeout)
	}
}

// Close disconnects from the broker and waits for the component's goroutines.
// It is safe to call more than once and on a component that never connected.
func (m *MQTT) Close() {
	m.mu.Lock()
	if m.phase == phaseClosed {
		m.mu.Unlock()
		return
	}
	m.phase = phaseClosed
	m.subscribed.Store(false)
	close(m.done)
	m.mu.Unlock()

	m.client.Disconnect(disconnectQuiesce)
	m.wg.Wait()
	log.Printf("mqtt: closed")
}

// Status returns the current connection and subscription state.
func (m *MQTT) Status() Status {
	return Status{
		Connected:  m.client.IsConnectionOpen(),
		Subscribed: m.subscribed.Load(),
	}
}

// IsConnected reports whether a broker connection is open.
func (m *MQTT) IsConnected() bool { return m.client.IsConnectionOpen() }

// Ready reports whether the connection is open and all subscriptions are
// registered.
func (m *MQTT) Ready() bool { return m.Status().Ready() }

// PublishStart sends a START command. See start.Publish for delivery semantics.
func (m *MQTT) PublishStart(ctx context.Context, payload start.Payload) error {
	if err := m.checkPublish(); err != nil {
		return err
	}
	return m.logPublishError(start.Publish(ctx, m.client, m.schemas, payload))
}

// PublishPermit sends a PERMIT command. See permit.Publish for delivery semantics.
func (m *MQTT) PublishPermit(ctx context.Context, payload permit.Payload) error {
	if err := m.checkPublish(); err != nil {
		return err
	}
	return m.logPublishError(permit.Publish(ctx, m.client, m.schemas, payload))
}

// PublishStop sends a STOP command. See stop.Publish for delivery semantics.
func (m *MQTT) PublishStop(ctx context.Context, payload stop.Payload) error {
	if err := m.checkPublish(); err != nil {
		return err
	}
	return m.logPublishError(stop.Publish(ctx, m.client, m.schemas, payload))
}

func (m *MQTT) checkPublish() error {
	m.mu.Lock()
	closed := m.phase == phaseClosed
	m.mu.Unlock()

	switch {
	case closed:
		return ErrClosed
	case !m.client.IsConnectionOpen():
		return ErrNotConnected
	}
	return nil
}

// logPublishError records publish failures here, at the component boundary,
// and still returns the error so the caller decides what to do with it.
func (m *MQTT) logPublishError(err error) error {
	if err != nil {
		log.Printf("mqtt: %v", err)
	}
	return err
}

func (m *MQTT) onConnect(client paho.Client) {
	m.mu.Lock()
	if m.phase == phaseClosed {
		m.mu.Unlock()
		return
	}
	m.generation++
	gen := m.generation
	m.wg.Add(1) // under mu so Close cannot reach wg.Wait concurrently
	m.mu.Unlock()

	log.Printf("mqtt: connected to %s", m.broker())

	// Paho runs this handler in its own goroutine, so waiting on subscription
	// tokens here does not block message delivery.
	defer m.wg.Done()
	m.subscribeUntilDone(client, gen)
}

// subscribeUntilDone registers the subscriptions, retrying with backoff until
// they all succeed, the connection changes, or the component is closed.
func (m *MQTT) subscribeUntilDone(client paho.Client, gen uint64) {
	backoff := time.Second
	for {
		err := subscribeAll(client, m.routes)
		if err == nil {
			m.markSubscribed(gen)
			return
		}
		log.Printf("mqtt: subscription registration failed, retrying in %s: %v", backoff, err)

		timer := time.NewTimer(backoff)
		select {
		case <-m.done:
			timer.Stop()
			return
		case <-timer.C:
		}
		if !m.currentGeneration(gen) {
			return
		}
		backoff = min(backoff*2, maxRetryBackoff)
	}
}

func (m *MQTT) markSubscribed(gen uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.generation != gen || m.phase == phaseClosed {
		return // connection changed meanwhile; a newer attempt owns the state
	}
	m.subscribed.Store(true)
	m.readyOnce.Do(func() { close(m.ready) })
	log.Printf("mqtt: all %d subscriptions registered", len(m.routes))
}

func (m *MQTT) currentGeneration(gen uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.generation == gen
}

func (m *MQTT) onConnectionLost(_ paho.Client, err error) {
	m.mu.Lock()
	m.generation++
	m.subscribed.Store(false)
	closed := m.phase == phaseClosed
	m.mu.Unlock()

	if !closed {
		log.Printf("mqtt: connection to %s lost: %v", m.broker(), err)
	}
}

// broker returns the broker address with any embedded credentials redacted.
func (m *MQTT) broker() string {
	u, err := url.Parse(m.cfg.Broker)
	if err != nil {
		return "<invalid broker address>"
	}
	return u.Redacted()
}
