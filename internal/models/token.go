package models

import "time"

// EnrollmentToken represents a one-time token used for device registration.
type EnrollmentToken struct {
	ID           string    `json:"id"`
	Token        string    `json:"token"`
	Label        string    `json:"label"`
	Code         string    `json:"code,omitempty"`
	DeviceName   string    `json:"device_name,omitempty"`
	AcceptSSH    bool      `json:"accept_ssh,omitempty"`
	SyncInterval string    `json:"sync_interval,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	Used         bool      `json:"used"`
	UsedBy       *string   `json:"used_by,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}
