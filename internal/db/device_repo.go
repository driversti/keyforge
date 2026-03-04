package db

import (
	"crypto/md5"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/driversti/keyforge/internal/models"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

// CreateDevice inserts a new device into the database and returns it.
func (d *DB) CreateDevice(req models.CreateDeviceRequest) (*models.Device, error) {
	id := uuid.New().String()
	fingerprint := computeFingerprint(req.PublicKey)

	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return nil, fmt.Errorf("marshal tags: %w", err)
	}

	now := time.Now().UTC()

	_, err = d.DB.Exec(
		`INSERT INTO devices (id, name, public_key, fingerprint, accepts_ssh, tags, status, registered_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, req.Name, req.PublicKey, fingerprint, req.AcceptsSSH, string(tagsJSON), string(models.StatusActive), now,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, fmt.Errorf("device with name %q already exists", req.Name)
		}
		return nil, fmt.Errorf("insert device: %w", err)
	}

	return &models.Device{
		ID:           id,
		Name:         req.Name,
		PublicKey:    req.PublicKey,
		Fingerprint:  fingerprint,
		AcceptsSSH:   req.AcceptsSSH,
		Tags:         tags,
		Status:       models.StatusActive,
		RegisteredAt: now,
	}, nil
}

// GetDevice retrieves a device by its ID.
func (d *DB) GetDevice(id string) (*models.Device, error) {
	row := d.DB.QueryRow(
		`SELECT id, name, public_key, fingerprint, accepts_ssh, tags, status, registered_at, last_seen
		 FROM devices WHERE id = ?`, id,
	)
	return scanDevice(row)
}

// GetDeviceByName retrieves a device by its unique name.
func (d *DB) GetDeviceByName(name string) (*models.Device, error) {
	row := d.DB.QueryRow(
		`SELECT id, name, public_key, fingerprint, accepts_ssh, tags, status, registered_at, last_seen
		 FROM devices WHERE name = ?`, name,
	)
	return scanDevice(row)
}

// ListDevices returns all devices ordered by registration date descending.
func (d *DB) ListDevices() ([]models.Device, error) {
	rows, err := d.DB.Query(
		`SELECT id, name, public_key, fingerprint, accepts_ssh, tags, status, registered_at, last_seen
		 FROM devices ORDER BY registered_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query devices: %w", err)
	}
	defer rows.Close()
	return scanDeviceRows(rows)
}

// UpdateDevice updates the specified fields of a device.
func (d *DB) UpdateDevice(id string, req models.UpdateDeviceRequest) error {
	var setClauses []string
	var args []any

	if req.Name != nil {
		setClauses = append(setClauses, "name = ?")
		args = append(args, *req.Name)
	}
	if req.AcceptsSSH != nil {
		setClauses = append(setClauses, "accepts_ssh = ?")
		args = append(args, *req.AcceptsSSH)
	}
	if req.Tags != nil {
		tagsJSON, err := json.Marshal(req.Tags)
		if err != nil {
			return fmt.Errorf("marshal tags: %w", err)
		}
		setClauses = append(setClauses, "tags = ?")
		args = append(args, string(tagsJSON))
	}

	if len(setClauses) == 0 {
		return nil
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE devices SET %s WHERE id = ?", strings.Join(setClauses, ", "))

	result, err := d.DB.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update device: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// RevokeDevice sets a device's status to revoked.
func (d *DB) RevokeDevice(id string) error {
	result, err := d.DB.Exec("UPDATE devices SET status = ? WHERE id = ?", string(models.StatusRevoked), id)
	if err != nil {
		return fmt.Errorf("revoke device: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ReactivateDevice sets a device's status back to active.
func (d *DB) ReactivateDevice(id string) error {
	result, err := d.DB.Exec("UPDATE devices SET status = ? WHERE id = ?", string(models.StatusActive), id)
	if err != nil {
		return fmt.Errorf("reactivate device: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteDevice removes a device from the database.
func (d *DB) DeleteDevice(id string) error {
	result, err := d.DB.Exec("DELETE FROM devices WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete device: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetActivePublicKeys returns all devices with status 'active'.
func (d *DB) GetActivePublicKeys() ([]models.Device, error) {
	rows, err := d.DB.Query(
		`SELECT id, name, public_key, fingerprint, accepts_ssh, tags, status, registered_at, last_seen
		 FROM devices WHERE status = ? ORDER BY registered_at DESC`,
		string(models.StatusActive),
	)
	if err != nil {
		return nil, fmt.Errorf("query active devices: %w", err)
	}
	defer rows.Close()
	return scanDeviceRows(rows)
}

// UpdateLastSeen sets the last_seen timestamp of a device to the current time.
func (d *DB) UpdateLastSeen(id string) error {
	result, err := d.DB.Exec("UPDATE devices SET last_seen = ? WHERE id = ?", time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("update last_seen: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// scanDevice scans a single device row into a Device struct.
func scanDevice(row *sql.Row) (*models.Device, error) {
	var dev models.Device
	var tagsJSON string
	var lastSeen sql.NullTime

	err := row.Scan(
		&dev.ID, &dev.Name, &dev.PublicKey, &dev.Fingerprint,
		&dev.AcceptsSSH, &tagsJSON, &dev.Status, &dev.RegisteredAt, &lastSeen,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan device: %w", err)
	}

	if err := json.Unmarshal([]byte(tagsJSON), &dev.Tags); err != nil {
		return nil, fmt.Errorf("unmarshal tags: %w", err)
	}
	if lastSeen.Valid {
		dev.LastSeen = &lastSeen.Time
	}

	return &dev, nil
}

// scanDeviceRows scans multiple device rows into a slice of Device structs.
func scanDeviceRows(rows *sql.Rows) ([]models.Device, error) {
	var devices []models.Device

	for rows.Next() {
		var dev models.Device
		var tagsJSON string
		var lastSeen sql.NullTime

		err := rows.Scan(
			&dev.ID, &dev.Name, &dev.PublicKey, &dev.Fingerprint,
			&dev.AcceptsSSH, &tagsJSON, &dev.Status, &dev.RegisteredAt, &lastSeen,
		)
		if err != nil {
			return nil, fmt.Errorf("scan device row: %w", err)
		}

		if err := json.Unmarshal([]byte(tagsJSON), &dev.Tags); err != nil {
			return nil, fmt.Errorf("unmarshal tags: %w", err)
		}
		if lastSeen.Valid {
			dev.LastSeen = &lastSeen.Time
		}

		devices = append(devices, dev)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate device rows: %w", err)
	}

	return devices, nil
}

// computeFingerprint computes the MD5 fingerprint of a public key as colon-separated hex.
func computeFingerprint(publicKey string) string {
	hash := md5.Sum([]byte(publicKey))
	parts := make([]string, len(hash))
	for i, b := range hash {
		parts[i] = fmt.Sprintf("%02x", b)
	}
	return strings.Join(parts, ":")
}

// isUniqueConstraintError checks if an error is a SQLite UNIQUE constraint violation.
func isUniqueConstraintError(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
