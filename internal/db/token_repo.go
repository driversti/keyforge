package db

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/driversti/keyforge/internal/models"
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
	err := d.DB.QueryRow(
		`SELECT id, token, label, expires_at, used, used_by, created_at FROM enrollment_tokens WHERE id = ?`,
		id,
	).Scan(&t.ID, &t.Token, &t.Label, &t.ExpiresAt, &t.Used, &t.UsedBy, &t.CreatedAt)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get token: %w", err)
	}
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
	err = tx.QueryRow(
		`SELECT id, token, label, expires_at, used, used_by, created_at FROM enrollment_tokens WHERE token = ?`,
		tokenValue,
	).Scan(&t.ID, &t.Token, &t.Label, &t.ExpiresAt, &t.Used, &t.UsedBy, &t.CreatedAt)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, fmt.Errorf("token not found")
		}
		return nil, fmt.Errorf("query token: %w", err)
	}

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
		`SELECT id, token, label, expires_at, used, used_by, created_at FROM enrollment_tokens ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}
	defer rows.Close()

	var tokens []models.EnrollmentToken
	for rows.Next() {
		var t models.EnrollmentToken
		if err := rows.Scan(&t.ID, &t.Token, &t.Label, &t.ExpiresAt, &t.Used, &t.UsedBy, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan token: %w", err)
		}
		tokens = append(tokens, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tokens: %w", err)
	}

	return tokens, nil
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
