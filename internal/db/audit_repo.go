package db

import (
	"fmt"

	"driversti.dev/keyforge/internal/models"
)

// LogAudit inserts a new entry into the audit log.
func (d *DB) LogAudit(action string, deviceID *string, details string, sourceIP string) error {
	_, err := d.DB.Exec(
		`INSERT INTO audit_log (action, device_id, details, source_ip) VALUES (?, ?, ?, ?)`,
		action, deviceID, details, sourceIP,
	)
	if err != nil {
		return fmt.Errorf("insert audit entry: %w", err)
	}
	return nil
}

// ListAudit returns audit log entries ordered by creation date descending,
// with the given limit and offset for pagination.
func (d *DB) ListAudit(limit, offset int) ([]models.AuditEntry, error) {
	rows, err := d.DB.Query(
		`SELECT id, action, device_id, details, source_ip, created_at
		 FROM audit_log ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("query audit log: %w", err)
	}
	defer rows.Close()

	var entries []models.AuditEntry
	for rows.Next() {
		var entry models.AuditEntry
		err := rows.Scan(
			&entry.ID, &entry.Action, &entry.DeviceID,
			&entry.Details, &entry.SourceIP, &entry.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan audit entry: %w", err)
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit rows: %w", err)
	}

	return entries, nil
}

// CountAudit returns the total number of audit log entries.
func (d *DB) CountAudit() (int, error) {
	var count int
	err := d.DB.QueryRow(`SELECT COUNT(*) FROM audit_log`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count audit entries: %w", err)
	}
	return count, nil
}
