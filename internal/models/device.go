package models

import "time"

// DeviceStatus represents the status of a registered device.
type DeviceStatus string

const (
	StatusActive  DeviceStatus = "active"
	StatusRevoked DeviceStatus = "revoked"
)

// Device represents a registered device and its SSH public key.
type Device struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	PublicKey    string       `json:"public_key"`
	Fingerprint  string       `json:"fingerprint"`
	AcceptsSSH   bool         `json:"accepts_ssh"`
	Tags         []string     `json:"tags"`
	Status       DeviceStatus `json:"status"`
	RegisteredAt time.Time    `json:"registered_at"`
	LastSeen     *time.Time   `json:"last_seen,omitempty"`
}

// CreateDeviceRequest is the payload for registering a new device.
type CreateDeviceRequest struct {
	Name            string   `json:"name"`
	PublicKey       string   `json:"public_key"`
	AcceptsSSH      bool     `json:"accepts_ssh"`
	Tags            []string `json:"tags"`
	EnrollmentToken string   `json:"enrollment_token,omitempty"`
}

// UpdateDeviceRequest is the payload for updating an existing device.
type UpdateDeviceRequest struct {
	Name       *string  `json:"name,omitempty"`
	AcceptsSSH *bool    `json:"accepts_ssh,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}
