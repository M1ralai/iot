package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/M1ralai/iot/src/server"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("irrigation backend: %v", err)
	}
}

func run() error {
	cfg, err := server.LoadConfig()
	if err != nil {
		return err
	}

	srv, err := server.New(cfg)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return srv.Run(ctx)
}
