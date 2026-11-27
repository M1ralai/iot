package status

type Payload struct {
	SchemaVersion    int    `json:"schema_version"`
	Kind             string `json:"kind"`
	GatewayID        string `json:"gateway_id"`
	GatewayBootID    string `json:"gateway_boot_id"`
	GatewaySessionID string `json:"gateway_session_id"`
	MessageID        string `json:"message_id"`
	State            string `json:"state"`
}
