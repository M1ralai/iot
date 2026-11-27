package main

import "github.com/M1ralai/iot/src/server"

func main() {
	cfg := server.NewServerConfig(":8080", "mock","mock")
	srv := server.NewServer(cfg)
	srv.Run()
}
