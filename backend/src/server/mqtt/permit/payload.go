package permit

import "time"

type Payload struct {
	SchemaVersion    int       `json:"schema_version"`
	Kind             string    `json:"kind"`
	GatewayID        string    `json:"gateway_id"`
	GatewayBootID    string    `json:"gateway_boot_id"`
	GatewaySessionID string    `json:"gateway_session_id"`
	MessageID        string    `json:"message_id"`
	NodeID           string    `json:"node_id"`
	BackendEpoch     string    `json:"backend_epoch"`
	CommandID        string    `json:"command_id"`
	RunID            string    `json:"run_id"`
	TargetBootID     string    `json:"target_boot_id"`
	IssuedAt         time.Time `json:"issued_at"`
	ExpiresAt        time.Time `json:"expires_at"`
	Payload          Command   `json:"payload"`
}

type Command struct {
	ArmNonce       string `json:"arm_nonce"`
	PermitBudgetMS uint32 `json:"permit_budget_ms"`
}
