package server

import (
	"errors"
	"os"

	"github.com/M1ralai/iot/src/server/mqtt"
)

// Config holds the settings of every server component.
type Config struct {
	RESTAddr string
	MQTT     mqtt.Config
}

// LoadConfig reads the configuration from environment variables:
//
//	IOT_REST_ADDR       REST listen address   (default ":8080")
//	IOT_MQTT_BROKER     broker URL            (default "tcp://localhost:1883")
//	IOT_MQTT_CLIENT_ID  MQTT client ID        (default "backend-irrigation")
//	IOT_MQTT_USERNAME   broker username       (optional)
//	IOT_MQTT_PASSWORD   broker password       (optional)
func LoadConfig() (Config, error) {
	cfg := Config{
		RESTAddr: env("IOT_REST_ADDR", ":8080"),
		MQTT: mqtt.Config{
			Broker:   env("IOT_MQTT_BROKER", "tcp://localhost:1883"),
			ClientID: env("IOT_MQTT_CLIENT_ID", "backend-irrigation"),
			Username: os.Getenv("IOT_MQTT_USERNAME"),
			Password: os.Getenv("IOT_MQTT_PASSWORD"),
		},
	}

	if cfg.MQTT.Password != "" && cfg.MQTT.Username == "" {
		return Config{}, errors.New("IOT_MQTT_PASSWORD is set without IOT_MQTT_USERNAME")
	}

	return cfg, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
