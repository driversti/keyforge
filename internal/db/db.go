package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

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

	return nil
}
