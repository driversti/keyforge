package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// DB wraps a *sql.DB connection to the SQLite database.
type DB struct {
	DB *sql.DB
}

// New opens a SQLite database at the given DSN, enables WAL mode and foreign
// keys, and runs schema migrations. For file-based DSNs it creates the parent
// directory if it does not already exist.
func New(dsn string) (*DB, error) {
	if dsn != ":memory:" {
		dir := filepath.Dir(dsn)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	// Enable foreign key constraint enforcement.
	if _, err := sqlDB.Exec("PRAGMA foreign_keys=ON"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	d := &DB{DB: sqlDB}
	if err := d.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return d, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.DB.Close()
}

// GetSetting retrieves a setting value by key from the settings table.
func (d *DB) GetSetting(key string) (string, error) {
	var value string
	err := d.DB.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		return "", fmt.Errorf("get setting %q: %w", key, err)
	}
	return value, nil
}

// DeleteSetting removes a setting by key from the settings table.
func (d *DB) DeleteSetting(key string) error {
	_, err := d.DB.Exec("DELETE FROM settings WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("delete setting %q: %w", key, err)
	}
	return nil
}

// SetSetting inserts or replaces a setting value in the settings table.
func (d *DB) SetSetting(key, value string) error {
	_, err := d.DB.Exec(
		"INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}

// migrate creates the application tables if they do not already exist.
func (d *DB) migrate() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS devices (
			id TEXT PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			public_key TEXT NOT NULL,
			fingerprint TEXT NOT NULL,
			accepts_ssh BOOLEAN NOT NULL DEFAULT false,
			tags TEXT NOT NULL DEFAULT '[]',
			status TEXT NOT NULL DEFAULT 'active',
			registered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_seen DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS enrollment_tokens (
			id TEXT PRIMARY KEY,
			token TEXT UNIQUE NOT NULL,
			label TEXT,
			code TEXT UNIQUE,
			expires_at DATETIME NOT NULL,
			used BOOLEAN NOT NULL DEFAULT false,
			used_by TEXT REFERENCES devices(id),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			action TEXT NOT NULL,
			device_id TEXT,
			details TEXT,
			source_ip TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}

	for _, stmt := range statements {
		if _, err := d.DB.Exec(stmt); err != nil {
			return fmt.Errorf("execute migration: %w", err)
		}
	}

	// Add new columns for quick enrollment URLs.
	alterColumns := []struct {
		table, column, definition string
	}{
		{"enrollment_tokens", "code", "TEXT"},
		{"enrollment_tokens", "device_name", "TEXT"},
		{"enrollment_tokens", "accept_ssh", "BOOLEAN NOT NULL DEFAULT false"},
		{"enrollment_tokens", "sync_interval", "TEXT"},
	}
	for _, col := range alterColumns {
		if err := d.addColumnIfNotExists(col.table, col.column, col.definition); err != nil {
			return fmt.Errorf("add column %s.%s: %w", col.table, col.column, err)
		}
	}

	// Ensure unique index on code (for existing DBs where ALTER TABLE added the column).
	_, _ = d.DB.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_enrollment_tokens_code ON enrollment_tokens(code)`)

	return nil
}

// addColumnIfNotExists adds a column to a table, ignoring "duplicate column" errors.
func (d *DB) addColumnIfNotExists(table, column, definition string) error {
	_, err := d.DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	if err != nil && strings.Contains(err.Error(), "duplicate column") {
		return nil
	}
	return err
}
