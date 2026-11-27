package server

import (
	"net/http"

	"github.com/M1ralai/iot/src/server/rest/middlewares/logger"
	"github.com/M1ralai/iot/src/server/rest/modules/health"
	"github.com/M1ralai/iot/src/server/mqtt"
)

type ServerCfg struct {
	port     string
	broker   string
	clientId string
}

func NewServerConfig(port string, broker string, clientId string) ServerCfg {
	return ServerCfg{
		port:     port,
		broker:   broker,
		clientId: clientId,
	}
}

type Server struct {
	mux *http.ServeMux
	cfg ServerCfg
	m   *mqtt.MQTT
}

func NewServer(cfg ServerCfg) *Server {
	mux := http.NewServeMux()
	m := mqtt.New(cfg.broker, cfg.clientId)
	return &Server{
		mux: mux,
		cfg: cfg,
		m:   m,
	}
}

func (s *Server) Run() {
	s.mux.HandleFunc("/health", logger.LoggingHandler(health.HealthHandler))
	http.ListenAndServe(s.cfg.port, s.mux)
}
