// Package server composes the REST and MQTT components and owns their
// lifecycle.
package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/M1ralai/iot/src/server/mqtt"
	"github.com/M1ralai/iot/src/server/mqtt/schema"
	"github.com/M1ralai/iot/src/server/rest"
	"github.com/M1ralai/iot/src/server/rest/modules/health"
)

const shutdownTimeout = 10 * time.Second

type Server struct {
	rest *rest.REST
	mqtt *mqtt.MQTT
}

// New compiles the embedded message schemas and builds the components. It
// performs no network I/O; a schema failure prevents startup.
func New(cfg Config) (*Server, error) {
	schemas, err := schema.Load()
	if err != nil {
		return nil, fmt.Errorf("load message schemas: %w", err)
	}

	m := mqtt.New(cfg.MQTT, schemas)

	mqttCheck := health.Check{
		Name: "mqtt",
		Status: func() map[string]bool {
			st := m.Status()
			return map[string]bool{"connected": st.Connected, "subscribed": st.Subscribed}
		},
	}

	return &Server{
		rest: rest.New(cfg.RESTAddr, mqttCheck),
		mqtt: m,
	}, nil
}

// MQTT exposes the MQTT component so application logic can publish commands.
func (s *Server) MQTT() *mqtt.MQTT { return s.mqtt }

// Run starts MQTT, then REST, and blocks until ctx is cancelled or a component
// fails, then shuts both down. If startup fails, components that already
// started are stopped. It returns nil on a clean, context-triggered shutdown.
func (s *Server) Run(ctx context.Context) error {
	if err := s.mqtt.Connect(ctx); err != nil {
		return fmt.Errorf("start mqtt: %w", err)
	}
	if err := s.rest.Start(); err != nil {
		s.mqtt.Close()
		return fmt.Errorf("start rest: %w", err)
	}
	log.Printf("server: started")

	var runErr error
	select {
	case <-ctx.Done():
		log.Printf("server: shutting down: %v", context.Cause(ctx))
	case err := <-s.rest.Err():
		runErr = err
		log.Printf("server: shutting down after component failure")
	}

	return errors.Join(runErr, s.shutdown())
}

// shutdown stops REST first so no request can use MQTT while it closes.
func (s *Server) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	err := s.rest.Shutdown(ctx)
	s.mqtt.Close()
	return err
}
