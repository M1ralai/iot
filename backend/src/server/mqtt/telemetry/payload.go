package telemetry

import "encoding/json"

type Payload struct {
	SchemaVersion     int             `json:"schema_version"`
	Kind              string          `json:"kind"`
	GatewayID         string          `json:"gateway_id"`
	GatewayBootID     string          `json:"gateway_boot_id"`
	GatewaySessionID  string          `json:"gateway_session_id"`
	MessageID         string          `json:"message_id"`
	NodeID            string          `json:"node_id"`
	Historical        bool            `json:"historical"`
	GatewayQueueAgeMS uint32          `json:"gateway_queue_age_ms"`
	LastHopRSSIDBm    *int            `json:"last_hop_rssi_dbm"`
	Frame             json.RawMessage `json:"frame"`
}
