package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"

	"driversti.dev/keyforge/internal/models"
)

// CreateToken generates a new enrollment token with the given label and expiry.
func (d *DB) CreateToken(label string, expiresAt time.Time) (*models.EnrollmentToken, error) {
	id := uuid.New().String()

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generate token value: %w", err)
	}
	tokenValue := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b)

	now := time.Now().UTC()

	_, err := d.DB.Exec(
		`INSERT INTO enrollment_tokens (id, token, label, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, tokenValue, label, expiresAt.UTC(), now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert enrollment token: %w", err)
	}

	return &models.EnrollmentToken{
		ID:        id,
		Token:     tokenValue,
		Label:     label,
		ExpiresAt: expiresAt.UTC(),
		Used:      false,
		CreatedAt: now,
	}, nil
}

// GetToken retrieves an enrollment token by its ID.
func (d *DB) GetToken(id string) (*models.EnrollmentToken, error) {
	var t models.EnrollmentToken
	var code, deviceName, syncInterval sql.NullString
	err := d.DB.QueryRow(
		`SELECT id, token, label, code, device_name, accept_ssh, sync_interval, expires_at, used, used_by, created_at FROM enrollment_tokens WHERE id = ?`,
		id,
	).Scan(&t.ID, &t.Token, &t.Label, &code, &deviceName, &t.AcceptSSH, &syncInterval, &t.ExpiresAt, &t.Used, &t.UsedBy, &t.CreatedAt)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get token: %w", err)
	}
	t.Code = code.String
	t.DeviceName = deviceName.String
	t.SyncInterval = syncInterval.String
	return &t, nil
}

// ValidateAndBurnToken looks up a token by its value (not ID), validates it
// is not expired or already used, and atomically marks it as used.
func (d *DB) ValidateAndBurnToken(tokenValue string) (*models.EnrollmentToken, error) {
	tx, err := d.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	var t models.EnrollmentToken
	var code, deviceName, syncInterval sql.NullString
	err = tx.QueryRow(
		`SELECT id, token, label, code, device_name, accept_ssh, sync_interval, expires_at, used, used_by, created_at FROM enrollment_tokens WHERE token = ?`,
		tokenValue,
	).Scan(&t.ID, &t.Token, &t.Label, &code, &deviceName, &t.AcceptSSH, &syncInterval, &t.ExpiresAt, &t.Used, &t.UsedBy, &t.CreatedAt)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, fmt.Errorf("token not found")
		}
		return nil, fmt.Errorf("query token: %w", err)
	}

	t.Code = code.String
	t.DeviceName = deviceName.String
	t.SyncInterval = syncInterval.String

	if t.Used {
		return nil, fmt.Errorf("token already used")
	}
	if time.Now().UTC().After(t.ExpiresAt) {
		return nil, fmt.Errorf("token expired")
	}

	_, err = tx.Exec(`UPDATE enrollment_tokens SET used = true WHERE id = ?`, t.ID)
	if err != nil {
		return nil, fmt.Errorf("burn token: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	t.Used = true
	return &t, nil
}

// ListTokens returns all enrollment tokens ordered by creation date descending.
func (d *DB) ListTokens() ([]models.EnrollmentToken, error) {
	rows, err := d.DB.Query(
		`SELECT id, token, label, code, device_name, accept_ssh, sync_interval, expires_at, used, used_by, created_at FROM enrollment_tokens ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}
	defer rows.Close()

	var tokens []models.EnrollmentToken
	for rows.Next() {
		var t models.EnrollmentToken
		var code, deviceName, syncInterval sql.NullString
		if err := rows.Scan(&t.ID, &t.Token, &t.Label, &code, &deviceName, &t.AcceptSSH, &syncInterval, &t.ExpiresAt, &t.Used, &t.UsedBy, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan token: %w", err)
		}
		t.Code = code.String
		t.DeviceName = deviceName.String
		t.SyncInterval = syncInterval.String
		tokens = append(tokens, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tokens: %w", err)
	}

	return tokens, nil
}

// generateCode creates a random 4-digit numeric string using crypto/rand.
func generateCode() (string, error) {
	code := make([]byte, 4)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", fmt.Errorf("generate random digit: %w", err)
		}
		code[i] = '0' + byte(n.Int64())
	}
	return string(code), nil
}

// CreateQuickEnroll creates a new enrollment token with a short numeric code
// and baked-in enrollment configuration.
func (d *DB) CreateQuickEnroll(deviceName string, acceptSSH bool, syncInterval string, expiresAt time.Time) (*models.EnrollmentToken, error) {
	id := uuid.New().String()

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generate token value: %w", err)
	}
	tokenValue := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b)

	// Generate a unique code with retry.
	var code string
	for attempt := 0; attempt < 10; attempt++ {
		candidate, err := generateCode()
		if err != nil {
			return nil, fmt.Errorf("generate code: %w", err)
		}
		// Check uniqueness among active (unused) tokens.
		var count int
		err = d.DB.QueryRow(
			`SELECT COUNT(*) FROM enrollment_tokens WHERE code = ? AND used = false`,
			candidate,
		).Scan(&count)
		if err != nil {
			return nil, fmt.Errorf("check code uniqueness: %w", err)
		}
		if count == 0 {
			code = candidate
			break
		}
	}
	if code == "" {
		return nil, fmt.Errorf("failed to generate unique code after 10 attempts")
	}

	now := time.Now().UTC()

	_, err := d.DB.Exec(
		`INSERT INTO enrollment_tokens (id, token, label, code, device_name, accept_ssh, sync_interval, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, tokenValue, "quick-enroll", code, deviceName, acceptSSH, syncInterval, expiresAt.UTC(), now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert quick enroll token: %w", err)
	}

	return &models.EnrollmentToken{
		ID:           id,
		Token:        tokenValue,
		Label:        "quick-enroll",
		Code:         code,
		DeviceName:   deviceName,
		AcceptSSH:    acceptSSH,
		SyncInterval: syncInterval,
		ExpiresAt:    expiresAt.UTC(),
		Used:         false,
		CreatedAt:    now,
	}, nil
}

// GetTokenByCode retrieves an enrollment token by its short numeric code.
func (d *DB) GetTokenByCode(code string) (*models.EnrollmentToken, error) {
	var t models.EnrollmentToken
	var dbCode, deviceName, syncInterval sql.NullString
	err := d.DB.QueryRow(
		`SELECT id, token, label, code, device_name, accept_ssh, sync_interval, expires_at, used, used_by, created_at
		 FROM enrollment_tokens WHERE code = ?`,
		code,
	).Scan(&t.ID, &t.Token, &t.Label, &dbCode, &deviceName, &t.AcceptSSH, &syncInterval, &t.ExpiresAt, &t.Used, &t.UsedBy, &t.CreatedAt)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get token by code: %w", err)
	}
	t.Code = dbCode.String
	t.DeviceName = deviceName.String
	t.SyncInterval = syncInterval.String
	return &t, nil
}

// DeleteToken removes an enrollment token by its ID.
func (d *DB) DeleteToken(id string) error {
	result, err := d.DB.Exec(`DELETE FROM enrollment_tokens WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete token: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
