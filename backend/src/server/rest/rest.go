// Package rest owns the HTTP server, its routes and its listener.
package rest

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/M1ralai/iot/src/server/rest/middlewares/logger"
	"github.com/M1ralai/iot/src/server/rest/modules/health"
)

type REST struct {
	srv     *http.Server
	started atomic.Bool
	running atomic.Bool
	done    chan struct{} // closed when Serve returns
	errCh   chan error    // receives an unexpected Serve failure
}

// New builds the HTTP server and its routes. It does not listen. The checks
// of other components are reported by /health alongside REST's own.
func New(addr string, checks ...health.Check) *REST {
	r := &REST{
		done:  make(chan struct{}),
		errCh: make(chan error, 1),
	}

	all := append([]health.Check{{
		Name:   "rest",
		Status: func() map[string]bool { return map[string]bool{"running": r.running.Load()} },
	}}, checks...)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", logger.LoggingHandler(health.Handler(all...)))

	r.srv = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return r
}

// Start binds the listener and serves in the background. A bind failure is
// returned directly; later serve failures arrive on Err.
func (r *REST) Start() error {
	if !r.started.CompareAndSwap(false, true) {
		return errors.New("rest: Start called more than once")
	}

	ln, err := net.Listen("tcp", r.srv.Addr)
	if err != nil {
		close(r.done)
		return fmt.Errorf("rest: listen on %s: %w", r.srv.Addr, err)
	}

	r.running.Store(true)
	log.Printf("rest: listening on %s", ln.Addr())

	go func() {
		defer close(r.done)
		err := r.srv.Serve(ln)
		r.running.Store(false)
		if !errors.Is(err, http.ErrServerClosed) {
			r.errCh <- fmt.Errorf("rest: serve: %w", err)
		}
	}()

	return nil
}

// Err delivers an unexpected failure of the running server.
func (r *REST) Err() <-chan error { return r.errCh }

// Running reports whether the server is accepting connections.
func (r *REST) Running() bool { return r.running.Load() }

// Shutdown stops accepting connections, drains in-flight requests until ctx
// expires, then forces them closed. It is a no-op if Start was never called.
func (r *REST) Shutdown(ctx context.Context) error {
	if !r.started.Load() {
		return nil
	}

	err := r.srv.Shutdown(ctx)
	if err != nil {
		err = errors.Join(fmt.Errorf("rest: shutdown: %w", err), r.srv.Close())
	}
	<-r.done
	log.Printf("rest: stopped")
	return err
}
