package models

import "time"

// AuditEntry represents a single entry in the audit log.
type AuditEntry struct {
	ID        int64     `json:"id"`
	Action    string    `json:"action"`
	DeviceID  *string   `json:"device_id,omitempty"`
	Details   string    `json:"details"`
	SourceIP  string    `json:"source_ip"`
	CreatedAt time.Time `json:"created_at"`
}
