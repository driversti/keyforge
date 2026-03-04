# KeyForge Phase 1 — Core MVP Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a working KeyForge server with SQLite storage, REST API for device/key CRUD, the `/authorized_keys` plaintext endpoint, CLI commands (`serve`, `device add/list/revoke/delete`, `keys`), basic Web UI (device list, add device form, authorized keys copy page), and API key authentication.

**Architecture:** Single Go binary using cobra for CLI, stdlib `net/http` for HTTP server, `modernc.org/sqlite` for embedded database, and `html/template` + htmx for the Web UI. All web assets are embedded via `embed.FS`. The binary runs in two modes: `serve` (starts HTTP server) and various CLI subcommands that talk to the server's API.

**Tech Stack:** Go 1.22+, cobra, modernc.org/sqlite, html/template, htmx 2.x, standard library net/http

**Spec:** `/Users/driversti/Projects/SPEC.md`

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `cmd/keyforge/main.go`
- Create: `Makefile`

**Step 1: Initialize Go module and install dependencies**

Run:
```bash
cd /Users/driversti/Projects/keyforge
go mod init github.com/driversti/keyforge
go get github.com/spf13/cobra@latest
go get modernc.org/sqlite@latest
go get github.com/google/uuid@latest
```

**Step 2: Create the CLI entrypoint with root + serve commands**

Create `cmd/keyforge/main.go`:
```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	serverAddr string
	apiKey     string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "keyforge",
		Short: "SSH public key registry",
	}

	rootCmd.PersistentFlags().StringVar(&serverAddr, "server", "http://localhost:8080", "KeyForge server address")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key for authentication")

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the KeyForge server",
		RunE:  runServe,
	}
	serveCmd.Flags().Int("port", 8080, "Port to listen on")
	serveCmd.Flags().String("data", "./keyforge-data", "Data directory path")

	rootCmd.AddCommand(serveCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runServe(cmd *cobra.Command, args []string) error {
	port, _ := cmd.Flags().GetInt("port")
	dataDir, _ := cmd.Flags().GetString("data")
	fmt.Printf("Starting KeyForge server on :%d (data: %s)\n", port, dataDir)
	return nil
}
```

**Step 3: Create Makefile**

Create `Makefile`:
```makefile
BINARY=keyforge
VERSION?=0.1.0

.PHONY: build run test clean

build:
	go build -o bin/$(BINARY) ./cmd/keyforge

run: build
	./bin/$(BINARY) serve

test:
	go test ./... -v

clean:
	rm -rf bin/
```

**Step 4: Verify it builds and runs**

Run: `cd /Users/driversti/Projects/keyforge && make build && ./bin/keyforge --help && ./bin/keyforge serve --help`
Expected: Help output showing root command and serve subcommand with flags.

**Step 5: Commit**

```bash
git init
git add -A
git commit -m "feat: project scaffolding with cobra CLI and serve command"
```

---

### Task 2: SQLite Database Layer

**Files:**
- Create: `internal/db/db.go` — database connection + migrations
- Create: `internal/db/db_test.go` — tests for DB initialization

**Step 1: Write the failing test**

Create `internal/db/db_test.go`:
```go
package db_test

import (
	"testing"

	"github.com/driversti/keyforge/internal/db"
)

func TestNewDB_CreatesTablesOnInit(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer database.Close()

	// Verify tables exist by querying sqlite_master
	tables := []string{"devices", "enrollment_tokens", "audit_log", "settings"}
	for _, table := range tables {
		var name string
		err := database.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %q does not exist: %v", table, err)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/driversti/Projects/keyforge && go test ./internal/db/ -v`
Expected: FAIL — package doesn't exist yet.

**Step 3: Write minimal implementation**

Create `internal/db/db.go`:
```go
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func New(dsn string) (*DB, error) {
	// For file-based DSNs, ensure parent directory exists
	if dsn != ":memory:" && dsn != "" {
		dir := filepath.Dir(dsn)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode for better concurrent read performance
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := sqlDB.Exec("PRAGMA foreign_keys=ON"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	d := &DB{sqlDB}
	if err := d.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return d, nil
}

func (d *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS devices (
		id TEXT PRIMARY KEY,
		name TEXT UNIQUE NOT NULL,
		public_key TEXT NOT NULL,
		fingerprint TEXT NOT NULL,
		accepts_ssh BOOLEAN NOT NULL DEFAULT false,
		tags TEXT NOT NULL DEFAULT '[]',
		status TEXT NOT NULL DEFAULT 'active',
		registered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_seen DATETIME
	);

	CREATE TABLE IF NOT EXISTS enrollment_tokens (
		id TEXT PRIMARY KEY,
		token TEXT UNIQUE NOT NULL,
		label TEXT,
		expires_at DATETIME NOT NULL,
		used BOOLEAN NOT NULL DEFAULT false,
		used_by TEXT REFERENCES devices(id),
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		action TEXT NOT NULL,
		device_id TEXT,
		details TEXT,
		source_ip TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	`

	_, err := d.Exec(schema)
	return err
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/driversti/Projects/keyforge && go test ./internal/db/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: SQLite database layer with auto-migration"
```

---

### Task 3: Models

**Files:**
- Create: `internal/models/device.go`
- Create: `internal/models/token.go`
- Create: `internal/models/audit.go`

**Step 1: Create device model**

Create `internal/models/device.go`:
```go
package models

import "time"

type DeviceStatus string

const (
	StatusActive  DeviceStatus = "active"
	StatusRevoked DeviceStatus = "revoked"
)

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

type CreateDeviceRequest struct {
	Name            string   `json:"name"`
	PublicKey       string   `json:"public_key"`
	AcceptsSSH      bool     `json:"accepts_ssh"`
	Tags            []string `json:"tags"`
	EnrollmentToken string   `json:"enrollment_token,omitempty"`
}

type UpdateDeviceRequest struct {
	Name       *string  `json:"name,omitempty"`
	AcceptsSSH *bool    `json:"accepts_ssh,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}
```

**Step 2: Create token model**

Create `internal/models/token.go`:
```go
package models

import "time"

type EnrollmentToken struct {
	ID        string    `json:"id"`
	Token     string    `json:"token"`
	Label     string    `json:"label"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	UsedBy    *string   `json:"used_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
```

**Step 3: Create audit model**

Create `internal/models/audit.go`:
```go
package models

import "time"

type AuditEntry struct {
	ID        int64     `json:"id"`
	Action    string    `json:"action"`
	DeviceID  *string   `json:"device_id,omitempty"`
	Details   string    `json:"details"`
	SourceIP  string    `json:"source_ip"`
	CreatedAt time.Time `json:"created_at"`
}
```

**Step 4: Verify it compiles**

Run: `cd /Users/driversti/Projects/keyforge && go build ./internal/models/`
Expected: No errors.

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: add domain models for device, token, and audit"
```

---

### Task 4: Device Repository (CRUD)

**Files:**
- Create: `internal/db/device_repo.go`
- Create: `internal/db/device_repo_test.go`

**Step 1: Write the failing tests**

Create `internal/db/device_repo_test.go`:
```go
package db_test

import (
	"testing"

	"github.com/driversti/keyforge/internal/db"
	"github.com/driversti/keyforge/internal/models"
)

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestCreateDevice(t *testing.T) {
	d := newTestDB(t)

	dev, err := d.CreateDevice(models.CreateDeviceRequest{
		Name:       "test-device",
		PublicKey:  "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest test@host",
		AcceptsSSH: true,
		Tags:       []string{"test", "linux"},
	})
	if err != nil {
		t.Fatalf("create device: %v", err)
	}

	if dev.Name != "test-device" {
		t.Errorf("name = %q, want %q", dev.Name, "test-device")
	}
	if dev.Status != models.StatusActive {
		t.Errorf("status = %q, want %q", dev.Status, models.StatusActive)
	}
	if dev.Fingerprint == "" {
		t.Error("fingerprint should not be empty")
	}
}

func TestCreateDevice_DuplicateName(t *testing.T) {
	d := newTestDB(t)

	_, err := d.CreateDevice(models.CreateDeviceRequest{
		Name:      "dupe",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest1 test@host",
	})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err = d.CreateDevice(models.CreateDeviceRequest{
		Name:      "dupe",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest2 test@host",
	})
	if err == nil {
		t.Error("expected error for duplicate name, got nil")
	}
}

func TestListDevices(t *testing.T) {
	d := newTestDB(t)

	d.CreateDevice(models.CreateDeviceRequest{Name: "dev1", PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest1 t@h"})
	d.CreateDevice(models.CreateDeviceRequest{Name: "dev2", PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest2 t@h"})

	devices, err := d.ListDevices()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(devices) != 2 {
		t.Errorf("got %d devices, want 2", len(devices))
	}
}

func TestGetDevice(t *testing.T) {
	d := newTestDB(t)

	created, _ := d.CreateDevice(models.CreateDeviceRequest{
		Name:      "findme",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest t@h",
	})

	found, err := d.GetDevice(created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if found.Name != "findme" {
		t.Errorf("name = %q, want %q", found.Name, "findme")
	}
}

func TestRevokeDevice(t *testing.T) {
	d := newTestDB(t)

	created, _ := d.CreateDevice(models.CreateDeviceRequest{
		Name:      "revokeme",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest t@h",
	})

	err := d.RevokeDevice(created.ID)
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}

	dev, _ := d.GetDevice(created.ID)
	if dev.Status != models.StatusRevoked {
		t.Errorf("status = %q, want %q", dev.Status, models.StatusRevoked)
	}
}

func TestDeleteDevice(t *testing.T) {
	d := newTestDB(t)

	created, _ := d.CreateDevice(models.CreateDeviceRequest{
		Name:      "deleteme",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest t@h",
	})

	err := d.DeleteDevice(created.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = d.GetDevice(created.ID)
	if err == nil {
		t.Error("expected error for deleted device, got nil")
	}
}

func TestGetActivePublicKeys(t *testing.T) {
	d := newTestDB(t)

	d.CreateDevice(models.CreateDeviceRequest{Name: "active1", PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKey1 t@h"})
	created2, _ := d.CreateDevice(models.CreateDeviceRequest{Name: "active2", PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKey2 t@h"})
	d.RevokeDevice(created2.ID)

	keys, err := d.GetActivePublicKeys()
	if err != nil {
		t.Fatalf("get keys: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("got %d keys, want 1 (revoked should be excluded)", len(keys))
	}
}

func TestUpdateDevice(t *testing.T) {
	d := newTestDB(t)

	created, _ := d.CreateDevice(models.CreateDeviceRequest{
		Name:       "original",
		PublicKey:  "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest t@h",
		AcceptsSSH: false,
	})

	newName := "updated"
	acceptsSSH := true
	err := d.UpdateDevice(created.ID, models.UpdateDeviceRequest{
		Name:       &newName,
		AcceptsSSH: &acceptsSSH,
		Tags:       []string{"new-tag"},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	dev, _ := d.GetDevice(created.ID)
	if dev.Name != "updated" {
		t.Errorf("name = %q, want %q", dev.Name, "updated")
	}
	if !dev.AcceptsSSH {
		t.Error("accepts_ssh should be true after update")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/driversti/Projects/keyforge && go test ./internal/db/ -v`
Expected: FAIL — methods don't exist yet.

**Step 3: Implement device repository**

Create `internal/db/device_repo.go`:
```go
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

var ErrNotFound = errors.New("not found")

func (d *DB) CreateDevice(req models.CreateDeviceRequest) (*models.Device, error) {
	id := uuid.New().String()
	fingerprint := computeFingerprint(req.PublicKey)

	tags, err := json.Marshal(req.Tags)
	if err != nil {
		return nil, fmt.Errorf("marshal tags: %w", err)
	}
	if req.Tags == nil {
		tags = []byte("[]")
	}

	now := time.Now().UTC()
	_, err = d.Exec(`
		INSERT INTO devices (id, name, public_key, fingerprint, accepts_ssh, tags, status, registered_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, req.Name, req.PublicKey, fingerprint, req.AcceptsSSH, string(tags), models.StatusActive, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, fmt.Errorf("device with name %q already exists", req.Name)
		}
		return nil, fmt.Errorf("insert device: %w", err)
	}

	return &models.Device{
		ID:           id,
		Name:         req.Name,
		PublicKey:     req.PublicKey,
		Fingerprint:  fingerprint,
		AcceptsSSH:   req.AcceptsSSH,
		Tags:         req.Tags,
		Status:       models.StatusActive,
		RegisteredAt: now,
	}, nil
}

func (d *DB) GetDevice(id string) (*models.Device, error) {
	row := d.QueryRow(`SELECT id, name, public_key, fingerprint, accepts_ssh, tags, status, registered_at, last_seen FROM devices WHERE id = ?`, id)
	return scanDevice(row)
}

func (d *DB) GetDeviceByName(name string) (*models.Device, error) {
	row := d.QueryRow(`SELECT id, name, public_key, fingerprint, accepts_ssh, tags, status, registered_at, last_seen FROM devices WHERE name = ?`, name)
	return scanDevice(row)
}

func (d *DB) ListDevices() ([]models.Device, error) {
	rows, err := d.Query(`SELECT id, name, public_key, fingerprint, accepts_ssh, tags, status, registered_at, last_seen FROM devices ORDER BY registered_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query devices: %w", err)
	}
	defer rows.Close()

	var devices []models.Device
	for rows.Next() {
		dev, err := scanDeviceRows(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, *dev)
	}
	return devices, rows.Err()
}

func (d *DB) UpdateDevice(id string, req models.UpdateDeviceRequest) error {
	var sets []string
	var args []any

	if req.Name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *req.Name)
	}
	if req.AcceptsSSH != nil {
		sets = append(sets, "accepts_ssh = ?")
		args = append(args, *req.AcceptsSSH)
	}
	if req.Tags != nil {
		tags, _ := json.Marshal(req.Tags)
		sets = append(sets, "tags = ?")
		args = append(args, string(tags))
	}

	if len(sets) == 0 {
		return nil
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE devices SET %s WHERE id = ?", strings.Join(sets, ", "))
	result, err := d.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update device: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (d *DB) RevokeDevice(id string) error {
	result, err := d.Exec(`UPDATE devices SET status = ? WHERE id = ?`, models.StatusRevoked, id)
	if err != nil {
		return fmt.Errorf("revoke device: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (d *DB) ReactivateDevice(id string) error {
	result, err := d.Exec(`UPDATE devices SET status = ? WHERE id = ?`, models.StatusActive, id)
	if err != nil {
		return fmt.Errorf("reactivate device: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (d *DB) DeleteDevice(id string) error {
	result, err := d.Exec(`DELETE FROM devices WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete device: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (d *DB) GetActivePublicKeys() ([]models.Device, error) {
	rows, err := d.Query(`SELECT id, name, public_key, fingerprint, accepts_ssh, tags, status, registered_at, last_seen FROM devices WHERE status = ?`, models.StatusActive)
	if err != nil {
		return nil, fmt.Errorf("query active keys: %w", err)
	}
	defer rows.Close()

	var devices []models.Device
	for rows.Next() {
		dev, err := scanDeviceRows(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, *dev)
	}
	return devices, rows.Err()
}

func (d *DB) UpdateLastSeen(id string) error {
	_, err := d.Exec(`UPDATE devices SET last_seen = ? WHERE id = ?`, time.Now().UTC(), id)
	return err
}

func scanDevice(row *sql.Row) (*models.Device, error) {
	var dev models.Device
	var tagsJSON string
	var lastSeen sql.NullTime

	err := row.Scan(&dev.ID, &dev.Name, &dev.PublicKey, &dev.Fingerprint, &dev.AcceptsSSH, &tagsJSON, &dev.Status, &dev.RegisteredAt, &lastSeen)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan device: %w", err)
	}

	json.Unmarshal([]byte(tagsJSON), &dev.Tags)
	if lastSeen.Valid {
		dev.LastSeen = &lastSeen.Time
	}
	return &dev, nil
}

func scanDeviceRows(rows *sql.Rows) (*models.Device, error) {
	var dev models.Device
	var tagsJSON string
	var lastSeen sql.NullTime

	err := rows.Scan(&dev.ID, &dev.Name, &dev.PublicKey, &dev.Fingerprint, &dev.AcceptsSSH, &tagsJSON, &dev.Status, &dev.RegisteredAt, &lastSeen)
	if err != nil {
		return nil, fmt.Errorf("scan device row: %w", err)
	}

	json.Unmarshal([]byte(tagsJSON), &dev.Tags)
	if lastSeen.Valid {
		dev.LastSeen = &lastSeen.Time
	}
	return &dev, nil
}

func computeFingerprint(publicKey string) string {
	hash := md5.Sum([]byte(publicKey))
	parts := make([]string, len(hash))
	for i, b := range hash {
		parts[i] = fmt.Sprintf("%02x", b)
	}
	return strings.Join(parts, ":")
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/driversti/Projects/keyforge && go test ./internal/db/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: device repository with CRUD operations"
```

---

### Task 5: Audit Log Repository

**Files:**
- Create: `internal/db/audit_repo.go`
- Create: `internal/db/audit_repo_test.go`

**Step 1: Write the failing test**

Create `internal/db/audit_repo_test.go`:
```go
package db_test

import (
	"testing"
)

func TestLogAuditEntry(t *testing.T) {
	d := newTestDB(t)

	err := d.LogAudit("device.enrolled", nil, "Device test-device enrolled", "127.0.0.1")
	if err != nil {
		t.Fatalf("log audit: %v", err)
	}

	entries, err := d.ListAudit(10, 0)
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Action != "device.enrolled" {
		t.Errorf("action = %q, want %q", entries[0].Action, "device.enrolled")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/driversti/Projects/keyforge && go test ./internal/db/ -v -run TestLogAudit`
Expected: FAIL

**Step 3: Implement audit repository**

Create `internal/db/audit_repo.go`:
```go
package db

import (
	"fmt"

	"github.com/driversti/keyforge/internal/models"
)

func (d *DB) LogAudit(action string, deviceID *string, details string, sourceIP string) error {
	_, err := d.Exec(`INSERT INTO audit_log (action, device_id, details, source_ip) VALUES (?, ?, ?, ?)`,
		action, deviceID, details, sourceIP)
	if err != nil {
		return fmt.Errorf("log audit: %w", err)
	}
	return nil
}

func (d *DB) ListAudit(limit, offset int) ([]models.AuditEntry, error) {
	rows, err := d.Query(`SELECT id, action, device_id, details, source_ip, created_at FROM audit_log ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query audit log: %w", err)
	}
	defer rows.Close()

	var entries []models.AuditEntry
	for rows.Next() {
		var e models.AuditEntry
		var deviceID *string
		if err := rows.Scan(&e.ID, &e.Action, &deviceID, &e.Details, &e.SourceIP, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audit entry: %w", err)
		}
		e.DeviceID = deviceID
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/driversti/Projects/keyforge && go test ./internal/db/ -v -run TestLogAudit`
Expected: PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: audit log repository"
```

---

### Task 6: Auth Middleware (API Key)

**Files:**
- Create: `internal/auth/auth.go`
- Create: `internal/auth/auth_test.go`

**Step 1: Write the failing test**

Create `internal/auth/auth_test.go`:
```go
package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/driversti/keyforge/internal/auth"
)

func TestAPIKeyMiddleware_ValidKey(t *testing.T) {
	handler := auth.RequireAPIKey("test-secret-key")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer test-secret-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAPIKeyMiddleware_MissingKey(t *testing.T) {
	handler := auth.RequireAPIKey("test-secret-key")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAPIKeyMiddleware_WrongKey(t *testing.T) {
	handler := auth.RequireAPIKey("test-secret-key")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/driversti/Projects/keyforge && go test ./internal/auth/ -v`
Expected: FAIL

**Step 3: Implement auth middleware**

Create `internal/auth/auth.go`:
```go
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
)

func RequireAPIKey(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := extractBearerToken(r)
			if provided == "" {
				http.Error(w, `{"error":"missing API key"}`, http.StatusUnauthorized)
				return
			}
			if subtle.ConstantTimeCompare([]byte(provided), []byte(apiKey)) != 1 {
				http.Error(w, `{"error":"invalid API key"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(auth, "Bearer ")
}

func GenerateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/driversti/Projects/keyforge && go test ./internal/auth/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: API key authentication middleware"
```

---

### Task 7: REST API Handlers

**Files:**
- Create: `internal/api/handlers.go`
- Create: `internal/api/handlers_test.go`

**Step 1: Write the failing tests**

Create `internal/api/handlers_test.go`:
```go
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/driversti/keyforge/internal/api"
	"github.com/driversti/keyforge/internal/db"
	"github.com/driversti/keyforge/internal/models"
)

func setup(t *testing.T) (*api.Handler, *db.DB) {
	t.Helper()
	d, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	h := api.NewHandler(d)
	return h, d
}

func TestGetAuthorizedKeys(t *testing.T) {
	h, d := setup(t)

	d.CreateDevice(models.CreateDeviceRequest{Name: "dev1", PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKey1 dev1"})
	d.CreateDevice(models.CreateDeviceRequest{Name: "dev2", PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKey2 dev2"})

	req := httptest.NewRequest("GET", "/api/v1/authorized_keys", nil)
	rec := httptest.NewRecorder()
	h.GetAuthorizedKeys(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Errorf("content-type = %q", rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	if !contains(body, "Key1") || !contains(body, "Key2") {
		t.Errorf("body missing keys: %s", body)
	}
}

func TestListDevices(t *testing.T) {
	h, d := setup(t)
	d.CreateDevice(models.CreateDeviceRequest{Name: "dev1", PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKey1 t@h"})

	req := httptest.NewRequest("GET", "/api/v1/devices", nil)
	rec := httptest.NewRecorder()
	h.ListDevices(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	var devices []models.Device
	json.Unmarshal(rec.Body.Bytes(), &devices)
	if len(devices) != 1 {
		t.Errorf("got %d devices, want 1", len(devices))
	}
}

func TestCreateDevice(t *testing.T) {
	h, _ := setup(t)

	body, _ := json.Marshal(models.CreateDeviceRequest{
		Name:      "new-dev",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINewKey t@h",
		Tags:      []string{"test"},
	})

	req := httptest.NewRequest("POST", "/api/v1/devices", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.CreateDevice(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var dev models.Device
	json.Unmarshal(rec.Body.Bytes(), &dev)
	if dev.Name != "new-dev" {
		t.Errorf("name = %q", dev.Name)
	}
}

func TestRevokeDevice(t *testing.T) {
	h, d := setup(t)
	created, _ := d.CreateDevice(models.CreateDeviceRequest{Name: "rev", PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKey t@h"})

	req := httptest.NewRequest("POST", "/api/v1/devices/"+created.ID+"/revoke", nil)
	rec := httptest.NewRecorder()
	h.RevokeDevice(rec, req, created.ID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestHealthCheck(t *testing.T) {
	h, _ := setup(t)
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	h.HealthCheck(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/driversti/Projects/keyforge && go test ./internal/api/ -v`
Expected: FAIL

**Step 3: Implement API handlers**

Create `internal/api/handlers.go`:
```go
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/driversti/keyforge/internal/db"
	"github.com/driversti/keyforge/internal/models"
)

type Handler struct {
	db *db.DB
}

func NewHandler(db *db.DB) *Handler {
	return &Handler{db: db}
}

func (h *Handler) GetAuthorizedKeys(w http.ResponseWriter, r *http.Request) {
	devices, err := h.db.GetActivePublicKeys()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	for _, dev := range devices {
		fmt.Fprintf(w, "%s\n", dev.PublicKey)
	}
}

func (h *Handler) ListDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := h.db.ListDevices()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if devices == nil {
		devices = []models.Device{}
	}
	writeJSON(w, http.StatusOK, devices)
}

func (h *Handler) GetDevice(w http.ResponseWriter, r *http.Request, id string) {
	dev, err := h.db.GetDevice(id)
	if err != nil {
		if err == db.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, dev)
}

func (h *Handler) CreateDevice(w http.ResponseWriter, r *http.Request) {
	var req models.CreateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" || req.PublicKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and public_key are required"})
		return
	}

	dev, err := h.db.CreateDevice(req)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	h.db.LogAudit("device.created", &dev.ID, fmt.Sprintf("Device %q registered", dev.Name), r.RemoteAddr)
	writeJSON(w, http.StatusCreated, dev)
}

func (h *Handler) UpdateDevice(w http.ResponseWriter, r *http.Request, id string) {
	var req models.UpdateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.db.UpdateDevice(id, req); err != nil {
		if err == db.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	dev, _ := h.db.GetDevice(id)
	h.db.LogAudit("device.updated", &id, fmt.Sprintf("Device %q updated", dev.Name), r.RemoteAddr)
	writeJSON(w, http.StatusOK, dev)
}

func (h *Handler) DeleteDevice(w http.ResponseWriter, r *http.Request, id string) {
	dev, err := h.db.GetDevice(id)
	if err != nil {
		if err == db.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if err := h.db.DeleteDevice(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	h.db.LogAudit("device.deleted", &id, fmt.Sprintf("Device %q deleted", dev.Name), r.RemoteAddr)
	writeJSON(w, http.StatusNoContent, nil)
}

func (h *Handler) RevokeDevice(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.db.RevokeDevice(id); err != nil {
		if err == db.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	h.db.LogAudit("device.revoked", &id, "Device revoked", r.RemoteAddr)

	dev, _ := h.db.GetDevice(id)
	writeJSON(w, http.StatusOK, dev)
}

func (h *Handler) ReactivateDevice(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.db.ReactivateDevice(id); err != nil {
		if err == db.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	h.db.LogAudit("device.reactivated", &id, "Device reactivated", r.RemoteAddr)

	dev, _ := h.db.GetDevice(id)
	writeJSON(w, http.StatusOK, dev)
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	if data == nil {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/driversti/Projects/keyforge && go test ./internal/api/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: REST API handlers for devices and authorized_keys"
```

---

### Task 8: HTTP Router & Server Wiring

**Files:**
- Create: `internal/server/server.go`
- Modify: `cmd/keyforge/main.go` — wire up server in `runServe`

**Step 1: Create server with routing**

Create `internal/server/server.go`:
```go
package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/driversti/keyforge/internal/api"
	"github.com/driversti/keyforge/internal/auth"
	"github.com/driversti/keyforge/internal/db"
)

type Server struct {
	db      *db.DB
	handler *api.Handler
	apiKey  string
	mux     *http.ServeMux
}

func New(database *db.DB, apiKey string) *Server {
	s := &Server{
		db:      database,
		handler: api.NewHandler(database),
		apiKey:  apiKey,
		mux:     http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	requireKey := auth.RequireAPIKey(s.apiKey)

	// Public endpoints (no auth)
	s.mux.HandleFunc("GET /api/v1/authorized_keys", s.handler.GetAuthorizedKeys)
	s.mux.HandleFunc("GET /api/v1/health", s.handler.HealthCheck)

	// Protected API endpoints
	s.mux.Handle("GET /api/v1/devices", requireKey(http.HandlerFunc(s.handler.ListDevices)))
	s.mux.Handle("POST /api/v1/devices", requireKey(http.HandlerFunc(s.handler.CreateDevice)))

	// Device-specific routes — extract ID from path
	s.mux.Handle("GET /api/v1/devices/{id}", requireKey(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.handler.GetDevice(w, r, r.PathValue("id"))
	})))
	s.mux.Handle("PATCH /api/v1/devices/{id}", requireKey(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.handler.UpdateDevice(w, r, r.PathValue("id"))
	})))
	s.mux.Handle("DELETE /api/v1/devices/{id}", requireKey(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.handler.DeleteDevice(w, r, r.PathValue("id"))
	})))
	s.mux.Handle("POST /api/v1/devices/{id}/revoke", requireKey(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.handler.RevokeDevice(w, r, r.PathValue("id"))
	})))
	s.mux.Handle("POST /api/v1/devices/{id}/reactivate", requireKey(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.handler.ReactivateDevice(w, r, r.PathValue("id"))
	})))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) ListenAndServe(port int) error {
	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("KeyForge server listening on %s\n", addr)
	return http.ListenAndServe(addr, s)
}
```

**Step 2: Update main.go to wire everything together**

Update `cmd/keyforge/main.go` — replace the `runServe` function and add the API key generation/loading logic. The full file should be:

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/driversti/keyforge/internal/auth"
	"github.com/driversti/keyforge/internal/db"
	"github.com/driversti/keyforge/internal/server"
)

var (
	serverAddr string
	apiKey     string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "keyforge",
		Short: "SSH public key registry",
	}

	rootCmd.PersistentFlags().StringVar(&serverAddr, "server", "http://localhost:8080", "KeyForge server address")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key for authentication")

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the KeyForge server",
		RunE:  runServe,
	}
	serveCmd.Flags().Int("port", 8080, "Port to listen on")
	serveCmd.Flags().String("data", "./keyforge-data", "Data directory path")

	rootCmd.AddCommand(serveCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runServe(cmd *cobra.Command, args []string) error {
	port, _ := cmd.Flags().GetInt("port")
	dataDir, _ := cmd.Flags().GetString("data")

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "keyforge.db")
	database, err := db.New(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	// Load or generate API key
	key, err := loadOrCreateAPIKey(database)
	if err != nil {
		return fmt.Errorf("API key setup: %w", err)
	}

	srv := server.New(database, key)
	return srv.ListenAndServe(port)
}

func loadOrCreateAPIKey(database *db.DB) (string, error) {
	var key string
	err := database.QueryRow("SELECT value FROM settings WHERE key = 'api_key'").Scan(&key)
	if err == nil {
		fmt.Println("API Key: (stored in database, use --api-key or check settings)")
		return key, nil
	}

	key, err = auth.GenerateAPIKey()
	if err != nil {
		return "", err
	}

	_, err = database.Exec("INSERT INTO settings (key, value) VALUES ('api_key', ?)", key)
	if err != nil {
		return "", err
	}

	fmt.Printf("\n=== FIRST RUN ===\nYour API key (save this!): %s\n=================\n\n", key)
	return key, nil
}
```

**Step 3: Verify it builds and starts**

Run: `cd /Users/driversti/Projects/keyforge && make build && timeout 2 ./bin/keyforge serve --data /tmp/keyforge-test || true`
Expected: Shows "FIRST RUN" with API key, then "KeyForge server listening on :8080".

**Step 4: Run all tests**

Run: `cd /Users/driversti/Projects/keyforge && go test ./... -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: HTTP router and server wiring with API key bootstrap"
```

---

### Task 9: CLI Device Commands

**Files:**
- Create: `cmd/keyforge/device.go` — device subcommands
- Create: `cmd/keyforge/keys.go` — keys command

**Step 1: Create device CLI subcommands**

Create `cmd/keyforge/device.go`:
```go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newDeviceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "device",
		Short: "Manage devices",
	}

	cmd.AddCommand(newDeviceListCmd())
	cmd.AddCommand(newDeviceAddCmd())
	cmd.AddCommand(newDeviceRevokeCmd())
	cmd.AddCommand(newDeviceReactivateCmd())
	cmd.AddCommand(newDeviceDeleteCmd())

	return cmd
}

func newDeviceListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all devices",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiRequest("GET", "/api/v1/devices", nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			var devices []map[string]any
			json.NewDecoder(resp.Body).Decode(&devices)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tSTATUS\tSSH\tFINGERPRINT\tREGISTERED")
			for _, d := range devices {
				acceptsSSH := "no"
				if v, ok := d["accepts_ssh"].(bool); ok && v {
					acceptsSSH = "yes"
				}
				fp := ""
				if v, ok := d["fingerprint"].(string); ok && len(v) > 17 {
					fp = v[:17] + "..."
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					d["name"], d["status"], acceptsSSH, fp, formatTime(d["registered_at"]))
			}
			w.Flush()
			return nil
		},
	}
}

func newDeviceAddCmd() *cobra.Command {
	var (
		name       string
		key        string
		acceptsSSH bool
		tags       string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a device manually",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" || key == "" {
				return fmt.Errorf("--name and --key are required")
			}

			var tagList []string
			if tags != "" {
				tagList = strings.Split(tags, ",")
			}

			body := map[string]any{
				"name":        name,
				"public_key":  key,
				"accepts_ssh": acceptsSSH,
				"tags":        tagList,
			}

			jsonBody, _ := json.Marshal(body)
			resp, err := apiRequest("POST", "/api/v1/devices", strings.NewReader(string(jsonBody)))
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				b, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("failed (%d): %s", resp.StatusCode, string(b))
			}

			fmt.Printf("Device %q added successfully\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Device name")
	cmd.Flags().StringVar(&key, "key", "", "SSH public key")
	cmd.Flags().BoolVar(&acceptsSSH, "accept-ssh", false, "Device accepts SSH connections")
	cmd.Flags().StringVar(&tags, "tags", "", "Comma-separated tags")

	return cmd
}

func newDeviceRevokeCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "revoke",
		Short: "Revoke a device",
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := resolveDeviceID(name)
			if err != nil {
				return err
			}

			resp, err := apiRequest("POST", "/api/v1/devices/"+id+"/revoke", nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("failed (%d): %s", resp.StatusCode, string(b))
			}

			fmt.Printf("Device %q revoked\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Device name")
	return cmd
}

func newDeviceReactivateCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "reactivate",
		Short: "Reactivate a revoked device",
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := resolveDeviceID(name)
			if err != nil {
				return err
			}

			resp, err := apiRequest("POST", "/api/v1/devices/"+id+"/reactivate", nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("failed (%d): %s", resp.StatusCode, string(b))
			}

			fmt.Printf("Device %q reactivated\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Device name")
	return cmd
}

func newDeviceDeleteCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Permanently delete a device",
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := resolveDeviceID(name)
			if err != nil {
				return err
			}

			resp, err := apiRequest("DELETE", "/api/v1/devices/"+id, nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusNoContent {
				b, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("failed (%d): %s", resp.StatusCode, string(b))
			}

			fmt.Printf("Device %q deleted\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Device name")
	return cmd
}

// resolveDeviceID finds a device ID by name via the API
func resolveDeviceID(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("--name is required")
	}

	resp, err := apiRequest("GET", "/api/v1/devices", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var devices []map[string]any
	json.NewDecoder(resp.Body).Decode(&devices)

	for _, d := range devices {
		if d["name"] == name {
			return d["id"].(string), nil
		}
	}
	return "", fmt.Errorf("device %q not found", name)
}

func apiRequest(method, path string, body io.Reader) (*http.Response, error) {
	url := strings.TrimRight(serverAddr, "/") + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return http.DefaultClient.Do(req)
}

func formatTime(v any) string {
	if s, ok := v.(string); ok && len(s) >= 10 {
		return s[:10]
	}
	return ""
}
```

**Step 2: Create keys command**

Create `cmd/keyforge/keys.go`:
```go
package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newKeysCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "keys",
		Short: "Print all active authorized keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiRequest("GET", "/api/v1/authorized_keys", nil)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			fmt.Print(string(body))
			return nil
		},
	}
}
```

**Step 3: Register commands in main.go**

Add to `main.go` after `rootCmd.AddCommand(serveCmd)`:
```go
	rootCmd.AddCommand(newDeviceCmd())
	rootCmd.AddCommand(newKeysCmd())
```

**Step 4: Verify it builds and shows help**

Run: `cd /Users/driversti/Projects/keyforge && make build && ./bin/keyforge device --help && ./bin/keyforge keys --help`
Expected: Help output showing device subcommands and keys command.

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: CLI commands for device management and keys"
```

---

### Task 10: Web UI — Templates & Static Assets

**Files:**
- Create: `web/static/style.css`
- Create: `web/templates/layout.html`
- Create: `web/templates/devices.html`
- Create: `web/templates/add_device.html`
- Create: `web/templates/authorized_keys.html`

**Step 1: Download htmx**

Run: `cd /Users/driversti/Projects/keyforge && mkdir -p web/static && curl -sL https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js -o web/static/htmx.min.js`

**Step 2: Create CSS**

Create `web/static/style.css`:
```css
:root {
    --bg: #0f172a;
    --surface: #1e293b;
    --border: #334155;
    --text: #e2e8f0;
    --text-muted: #94a3b8;
    --primary: #38bdf8;
    --primary-hover: #7dd3fc;
    --danger: #f87171;
    --success: #4ade80;
    --warning: #fbbf24;
}

* { box-sizing: border-box; margin: 0; padding: 0; }

body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif;
    background: var(--bg);
    color: var(--text);
    line-height: 1.6;
}

.container { max-width: 1200px; margin: 0 auto; padding: 0 1rem; }

nav {
    background: var(--surface);
    border-bottom: 1px solid var(--border);
    padding: 0.75rem 0;
    margin-bottom: 2rem;
}

nav .container { display: flex; align-items: center; gap: 2rem; }
nav .logo { font-size: 1.25rem; font-weight: 700; color: var(--primary); text-decoration: none; }
nav a { color: var(--text-muted); text-decoration: none; font-size: 0.9rem; }
nav a:hover, nav a.active { color: var(--primary); }

h1 { font-size: 1.5rem; margin-bottom: 1rem; }
h2 { font-size: 1.2rem; margin-bottom: 0.75rem; }

table { width: 100%; border-collapse: collapse; background: var(--surface); border-radius: 8px; overflow: hidden; }
th, td { padding: 0.75rem 1rem; text-align: left; border-bottom: 1px solid var(--border); }
th { font-size: 0.8rem; text-transform: uppercase; color: var(--text-muted); font-weight: 600; }
tr:last-child td { border-bottom: none; }
tr:hover { background: rgba(56, 189, 248, 0.05); }

.badge {
    display: inline-block; padding: 0.15rem 0.5rem; border-radius: 4px;
    font-size: 0.75rem; font-weight: 600;
}
.badge-active { background: rgba(74, 222, 128, 0.15); color: var(--success); }
.badge-revoked { background: rgba(248, 113, 113, 0.15); color: var(--danger); }

.btn {
    display: inline-block; padding: 0.5rem 1rem; border: none; border-radius: 6px;
    font-size: 0.85rem; font-weight: 500; cursor: pointer; text-decoration: none;
    transition: background 0.2s;
}
.btn-primary { background: var(--primary); color: var(--bg); }
.btn-primary:hover { background: var(--primary-hover); }
.btn-danger { background: transparent; border: 1px solid var(--danger); color: var(--danger); }
.btn-danger:hover { background: rgba(248, 113, 113, 0.15); }
.btn-sm { padding: 0.25rem 0.5rem; font-size: 0.75rem; }

.form-group { margin-bottom: 1rem; }
.form-group label { display: block; margin-bottom: 0.25rem; font-size: 0.85rem; color: var(--text-muted); }
.form-group input, .form-group textarea, .form-group select {
    width: 100%; padding: 0.5rem 0.75rem; background: var(--bg);
    border: 1px solid var(--border); border-radius: 6px; color: var(--text);
    font-size: 0.9rem; font-family: inherit;
}
.form-group textarea { min-height: 80px; resize: vertical; font-family: monospace; }
.form-group input:focus, .form-group textarea:focus {
    outline: none; border-color: var(--primary);
}

.checkbox-group { display: flex; align-items: center; gap: 0.5rem; }
.checkbox-group input[type="checkbox"] { width: auto; }

.card { background: var(--surface); border-radius: 8px; padding: 1.5rem; margin-bottom: 1rem; }

.keys-output {
    background: var(--bg); border: 1px solid var(--border); border-radius: 6px;
    padding: 1rem; font-family: monospace; font-size: 0.8rem; white-space: pre-wrap;
    word-break: break-all; max-height: 400px; overflow-y: auto; position: relative;
}

.copy-btn { position: absolute; top: 0.5rem; right: 0.5rem; }

.actions { display: flex; gap: 0.5rem; }
.top-bar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; }

.flash { padding: 0.75rem 1rem; border-radius: 6px; margin-bottom: 1rem; font-size: 0.9rem; }
.flash-success { background: rgba(74, 222, 128, 0.15); color: var(--success); }
.flash-error { background: rgba(248, 113, 113, 0.15); color: var(--danger); }

.fingerprint { font-family: monospace; font-size: 0.8rem; color: var(--text-muted); }

.tags { display: flex; gap: 0.25rem; flex-wrap: wrap; }
.tag {
    background: rgba(56, 189, 248, 0.15); color: var(--primary);
    padding: 0.1rem 0.4rem; border-radius: 3px; font-size: 0.7rem;
}

.key-count { font-size: 2rem; font-weight: 700; color: var(--primary); }
.stat-label { font-size: 0.85rem; color: var(--text-muted); }

@media (max-width: 768px) {
    nav .container { flex-wrap: wrap; gap: 0.5rem; }
    table { font-size: 0.85rem; }
    th, td { padding: 0.5rem; }
}
```

**Step 3: Create layout template**

Create `web/templates/layout.html`:
```html
{{define "layout"}}
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>KeyForge — {{template "title" .}}</title>
    <link rel="stylesheet" href="/static/style.css">
    <script src="/static/htmx.min.js"></script>
</head>
<body>
    <nav>
        <div class="container">
            <a href="/" class="logo">KeyForge</a>
            <a href="/">Devices</a>
            <a href="/add">Add Device</a>
            <a href="/authorized-keys">Authorized Keys</a>
        </div>
    </nav>
    <div class="container">
        {{template "content" .}}
    </div>
</body>
</html>
{{end}}
```

**Step 4: Create devices page template**

Create `web/templates/devices.html`:
```html
{{define "title"}}Devices{{end}}

{{define "content"}}
<div class="top-bar">
    <h1>Devices</h1>
    <a href="/add" class="btn btn-primary">+ Add Device</a>
</div>

{{if .Flash}}
<div class="flash flash-success">{{.Flash}}</div>
{{end}}

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Status</th>
            <th>SSH</th>
            <th>Fingerprint</th>
            <th>Tags</th>
            <th>Registered</th>
            <th>Actions</th>
        </tr>
    </thead>
    <tbody>
        {{range .Devices}}
        <tr>
            <td><strong>{{.Name}}</strong></td>
            <td>
                {{if eq (print .Status) "active"}}
                <span class="badge badge-active">active</span>
                {{else}}
                <span class="badge badge-revoked">revoked</span>
                {{end}}
            </td>
            <td>{{if .AcceptsSSH}}yes{{else}}no{{end}}</td>
            <td><span class="fingerprint">{{truncate .Fingerprint 20}}</span></td>
            <td>
                <div class="tags">
                {{range .Tags}}<span class="tag">{{.}}</span>{{end}}
                </div>
            </td>
            <td>{{formatDate .RegisteredAt}}</td>
            <td>
                <div class="actions">
                    {{if eq (print .Status) "active"}}
                    <form method="POST" action="/devices/{{.ID}}/revoke" style="display:inline">
                        <button type="submit" class="btn btn-danger btn-sm">Revoke</button>
                    </form>
                    {{else}}
                    <form method="POST" action="/devices/{{.ID}}/reactivate" style="display:inline">
                        <button type="submit" class="btn btn-primary btn-sm">Reactivate</button>
                    </form>
                    {{end}}
                    <form method="POST" action="/devices/{{.ID}}/delete" style="display:inline"
                          onsubmit="return confirm('Delete this device permanently?')">
                        <button type="submit" class="btn btn-danger btn-sm">Delete</button>
                    </form>
                </div>
            </td>
        </tr>
        {{else}}
        <tr><td colspan="7" style="text-align:center; color: var(--text-muted)">No devices registered yet</td></tr>
        {{end}}
    </tbody>
</table>
{{end}}
```

**Step 5: Create add device page template**

Create `web/templates/add_device.html`:
```html
{{define "title"}}Add Device{{end}}

{{define "content"}}
<h1>Add Device</h1>

{{if .Error}}
<div class="flash flash-error">{{.Error}}</div>
{{end}}

<div class="card">
    <form method="POST" action="/add">
        <div class="form-group">
            <label for="name">Device Name</label>
            <input type="text" id="name" name="name" placeholder="e.g. pixel-8, lxc-nginx" required value="{{.Form.Name}}">
        </div>

        <div class="form-group">
            <label for="public_key">SSH Public Key</label>
            <textarea id="public_key" name="public_key" placeholder="ssh-ed25519 AAAA... comment" required>{{.Form.PublicKey}}</textarea>
        </div>

        <div class="form-group">
            <label for="tags">Tags (comma-separated)</label>
            <input type="text" id="tags" name="tags" placeholder="e.g. linux, home-lab, ephemeral" value="{{.Form.Tags}}">
        </div>

        <div class="form-group checkbox-group">
            <input type="checkbox" id="accepts_ssh" name="accepts_ssh" {{if .Form.AcceptsSSH}}checked{{end}}>
            <label for="accepts_ssh">This device accepts incoming SSH connections</label>
        </div>

        <button type="submit" class="btn btn-primary">Add Device</button>
    </form>
</div>
{{end}}
```

**Step 6: Create authorized keys page template**

Create `web/templates/authorized_keys.html`:
```html
{{define "title"}}Authorized Keys{{end}}

{{define "content"}}
<div class="top-bar">
    <div>
        <h1>Authorized Keys</h1>
        <p style="color: var(--text-muted); font-size: 0.9rem">
            Copy this and paste into Proxmox or your server's authorized_keys file
        </p>
    </div>
    <div>
        <span class="key-count">{{.KeyCount}}</span>
        <span class="stat-label">active keys</span>
    </div>
</div>

<div class="card" style="position: relative">
    <button class="btn btn-primary btn-sm copy-btn" onclick="copyKeys()">Copy All</button>
    <div class="keys-output" id="keys-content">{{.Keys}}</div>
</div>

<div class="card">
    <h2>Quick Install</h2>
    <p style="color: var(--text-muted); font-size: 0.85rem; margin-bottom: 0.5rem">
        Run this on any server to add all keys:
    </p>
    <div class="keys-output">curl -s {{.ServerURL}}/api/v1/authorized_keys >> ~/.ssh/authorized_keys</div>
</div>

<script>
function copyKeys() {
    const text = document.getElementById('keys-content').textContent;
    navigator.clipboard.writeText(text).then(() => {
        const btn = document.querySelector('.copy-btn');
        btn.textContent = 'Copied!';
        setTimeout(() => btn.textContent = 'Copy All', 2000);
    });
}
</script>
{{end}}
```

**Step 7: Verify files exist**

Run: `cd /Users/driversti/Projects/keyforge && find web/ -type f | sort`
Expected: All template and static files listed.

**Step 8: Commit**

```bash
git add -A
git commit -m "feat: web UI templates and static assets (htmx + CSS)"
```

---

### Task 11: Web UI Handlers

**Files:**
- Create: `internal/web/web.go` — web handlers and template rendering
- Modify: `internal/server/server.go` — add web routes

**Step 1: Create web handlers**

Create `internal/web/web.go`:
```go
package web

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/driversti/keyforge/internal/db"
	"github.com/driversti/keyforge/internal/models"
)

//go:embed all:templates all:static
var content embed.FS

// Re-export for server to use
var StaticFS = content

type Handler struct {
	db        *db.DB
	templates *template.Template
	serverURL string
}

type addForm struct {
	Name       string
	PublicKey  string
	Tags       string
	AcceptsSSH bool
}

func NewHandler(database *db.DB, serverURL string) (*Handler, error) {
	funcMap := template.FuncMap{
		"truncate": func(s string, n int) string {
			if len(s) <= n {
				return s
			}
			return s[:n] + "..."
		},
		"formatDate": func(t time.Time) string {
			return t.Format("2006-01-02")
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(content, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	return &Handler{db: database, templates: tmpl, serverURL: serverURL}, nil
}

func (h *Handler) DevicesPage(w http.ResponseWriter, r *http.Request) {
	devices, err := h.db.ListDevices()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	flash := r.URL.Query().Get("flash")

	data := map[string]any{
		"Devices": devices,
		"Flash":   flash,
	}

	h.render(w, "layout", data)
}

func (h *Handler) AddDevicePage(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Form":  addForm{},
		"Error": "",
	}
	h.render(w, "layout", data)
}

func (h *Handler) AddDeviceSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	form := addForm{
		Name:       r.FormValue("name"),
		PublicKey:  strings.TrimSpace(r.FormValue("public_key")),
		Tags:       r.FormValue("tags"),
		AcceptsSSH: r.FormValue("accepts_ssh") == "on",
	}

	if form.Name == "" || form.PublicKey == "" {
		data := map[string]any{"Form": form, "Error": "Name and public key are required"}
		h.render(w, "layout", data)
		return
	}

	var tags []string
	if form.Tags != "" {
		for _, t := range strings.Split(form.Tags, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}

	_, err := h.db.CreateDevice(models.CreateDeviceRequest{
		Name:       form.Name,
		PublicKey:  form.PublicKey,
		AcceptsSSH: form.AcceptsSSH,
		Tags:       tags,
	})
	if err != nil {
		data := map[string]any{"Form": form, "Error": err.Error()}
		h.render(w, "layout", data)
		return
	}

	h.db.LogAudit("device.created", nil, fmt.Sprintf("Device %q added via Web UI", form.Name), r.RemoteAddr)
	http.Redirect(w, r, "/?flash=Device+added+successfully", http.StatusSeeOther)
}

func (h *Handler) AuthorizedKeysPage(w http.ResponseWriter, r *http.Request) {
	devices, err := h.db.GetActivePublicKeys()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var keys []string
	for _, d := range devices {
		keys = append(keys, d.PublicKey)
	}

	data := map[string]any{
		"Keys":      strings.Join(keys, "\n"),
		"KeyCount":  len(keys),
		"ServerURL": h.serverURL,
	}
	h.render(w, "layout", data)
}

func (h *Handler) RevokeDeviceAction(w http.ResponseWriter, r *http.Request, id string) {
	h.db.RevokeDevice(id)
	h.db.LogAudit("device.revoked", &id, "Revoked via Web UI", r.RemoteAddr)
	http.Redirect(w, r, "/?flash=Device+revoked", http.StatusSeeOther)
}

func (h *Handler) ReactivateDeviceAction(w http.ResponseWriter, r *http.Request, id string) {
	h.db.ReactivateDevice(id)
	h.db.LogAudit("device.reactivated", &id, "Reactivated via Web UI", r.RemoteAddr)
	http.Redirect(w, r, "/?flash=Device+reactivated", http.StatusSeeOther)
}

func (h *Handler) DeleteDeviceAction(w http.ResponseWriter, r *http.Request, id string) {
	h.db.DeleteDevice(id)
	h.db.LogAudit("device.deleted", &id, "Deleted via Web UI", r.RemoteAddr)
	http.Redirect(w, r, "/?flash=Device+deleted", http.StatusSeeOther)
}

func (h *Handler) render(w http.ResponseWriter, name string, data any) {
	// Determine which content template to use based on the request
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}
```

**Step 2: Update server.go to add web routes**

Add to `internal/server/server.go` — update the `New` function and `routes` method:

The updated `internal/server/server.go`:
```go
package server

import (
	"fmt"
	"io/fs"
	"net/http"

	"github.com/driversti/keyforge/internal/api"
	"github.com/driversti/keyforge/internal/auth"
	"github.com/driversti/keyforge/internal/db"
	"github.com/driversti/keyforge/internal/web"
)

type Server struct {
	db         *db.DB
	apiHandler *api.Handler
	webHandler *web.Handler
	apiKey     string
	mux        *http.ServeMux
}

func New(database *db.DB, apiKey string, serverURL string) (*Server, error) {
	webHandler, err := web.NewHandler(database, serverURL)
	if err != nil {
		return nil, fmt.Errorf("create web handler: %w", err)
	}

	s := &Server{
		db:         database,
		apiHandler: api.NewHandler(database),
		webHandler: webHandler,
		apiKey:     apiKey,
		mux:        http.NewServeMux(),
	}
	s.routes()
	return s, nil
}

func (s *Server) routes() {
	requireKey := auth.RequireAPIKey(s.apiKey)

	// --- API Routes ---
	s.mux.HandleFunc("GET /api/v1/authorized_keys", s.apiHandler.GetAuthorizedKeys)
	s.mux.HandleFunc("GET /api/v1/health", s.apiHandler.HealthCheck)

	s.mux.Handle("GET /api/v1/devices", requireKey(http.HandlerFunc(s.apiHandler.ListDevices)))
	s.mux.Handle("POST /api/v1/devices", requireKey(http.HandlerFunc(s.apiHandler.CreateDevice)))

	s.mux.Handle("GET /api/v1/devices/{id}", requireKey(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.apiHandler.GetDevice(w, r, r.PathValue("id"))
	})))
	s.mux.Handle("PATCH /api/v1/devices/{id}", requireKey(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.apiHandler.UpdateDevice(w, r, r.PathValue("id"))
	})))
	s.mux.Handle("DELETE /api/v1/devices/{id}", requireKey(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.apiHandler.DeleteDevice(w, r, r.PathValue("id"))
	})))
	s.mux.Handle("POST /api/v1/devices/{id}/revoke", requireKey(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.apiHandler.RevokeDevice(w, r, r.PathValue("id"))
	})))
	s.mux.Handle("POST /api/v1/devices/{id}/reactivate", requireKey(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.apiHandler.ReactivateDevice(w, r, r.PathValue("id"))
	})))

	// --- Web UI Routes ---
	s.mux.HandleFunc("GET /{$}", s.webHandler.DevicesPage)
	s.mux.HandleFunc("GET /add", s.webHandler.AddDevicePage)
	s.mux.HandleFunc("POST /add", s.webHandler.AddDeviceSubmit)
	s.mux.HandleFunc("GET /authorized-keys", s.webHandler.AuthorizedKeysPage)

	s.mux.HandleFunc("POST /devices/{id}/revoke", func(w http.ResponseWriter, r *http.Request) {
		s.webHandler.RevokeDeviceAction(w, r, r.PathValue("id"))
	})
	s.mux.HandleFunc("POST /devices/{id}/reactivate", func(w http.ResponseWriter, r *http.Request) {
		s.webHandler.ReactivateDeviceAction(w, r, r.PathValue("id"))
	})
	s.mux.HandleFunc("POST /devices/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		s.webHandler.DeleteDeviceAction(w, r, r.PathValue("id"))
	})

	// --- Static files ---
	staticFS, _ := fs.Sub(web.StaticFS, "static")
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) ListenAndServe(port int) error {
	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("KeyForge server listening on %s\n", addr)
	fmt.Printf("Web UI: http://localhost:%d\n", port)
	return http.ListenAndServe(addr, s)
}
```

**Step 3: Update main.go — pass serverURL to server.New**

In `cmd/keyforge/main.go`, update the `runServe` function — change the `server.New` call:
```go
	srv, err := server.New(database, key, fmt.Sprintf("http://localhost:%d", port))
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}
	return srv.ListenAndServe(port)
```

**Step 4: Move embedded FS to the web package**

Note: The `//go:embed` directive in `internal/web/web.go` references `templates` and `static` directories. These need to be under `internal/web/`, NOT under `web/`. Move the directories:

Run:
```bash
cd /Users/driversti/Projects/keyforge
mv web/templates internal/web/templates
mv web/static internal/web/static
rmdir web
```

**Step 5: Verify build and run**

Run: `cd /Users/driversti/Projects/keyforge && make build && timeout 3 ./bin/keyforge serve --data /tmp/keyforge-test2 || true`
Expected: Server starts, shows Web UI URL.

**Step 6: Run all tests**

Run: `cd /Users/driversti/Projects/keyforge && go test ./... -v`
Expected: ALL PASS

**Step 7: Commit**

```bash
git add -A
git commit -m "feat: Web UI with device list, add device form, and authorized keys page"
```

---

### Task 12: Web UI Template Fix — Content Routing

The layout template uses `{{template "content" .}}` but we need the right content template to render per page. We need to use separate template sets per page or use a page-name approach.

**Step 1: Update the handler to use per-page rendering**

In `internal/web/web.go`, update the `render` method and page handlers to explicitly set which content template to use. Each page handler should pass a `"Page"` key, and the layout should switch on it. Alternatively, parse each page template as a clone of the layout.

Replace the template parsing in `NewHandler`:
```go
func NewHandler(database *db.DB, serverURL string) (*Handler, error) {
	funcMap := template.FuncMap{
		"truncate": func(s string, n int) string {
			if len(s) <= n { return s }
			return s[:n] + "..."
		},
		"formatDate": func(t time.Time) string {
			return t.Format("2006-01-02")
		},
	}

	h := &Handler{db: database, serverURL: serverURL, funcMap: funcMap}
	return h, nil
}
```

Add a `funcMap` field to the struct and a method to render specific pages:
```go
type Handler struct {
	db        *db.DB
	funcMap   template.FuncMap
	serverURL string
}

func (h *Handler) renderPage(w http.ResponseWriter, page string, data any) {
	tmpl, err := template.New("").Funcs(h.funcMap).ParseFS(content, "templates/layout.html", "templates/"+page)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "render error: "+err.Error(), http.StatusInternalServerError)
	}
}
```

Then update each page handler to call `h.renderPage(w, "devices.html", data)`, `h.renderPage(w, "add_device.html", data)`, etc. instead of `h.render(w, "layout", data)`.

**Step 2: Verify build**

Run: `cd /Users/driversti/Projects/keyforge && make build`
Expected: Builds successfully.

**Step 3: Commit**

```bash
git add -A
git commit -m "fix: template rendering uses per-page template sets"
```

---

### Task 13: Integration Smoke Test

**Files:**
- Create: `test/integration_test.go`

**Step 1: Write integration test**

Create `test/integration_test.go`:
```go
package test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/driversti/keyforge/internal/db"
	"github.com/driversti/keyforge/internal/server"
)

func TestIntegration_FullWorkflow(t *testing.T) {
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	srv, err := server.New(database, "test-api-key", "http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	client := ts.Client()

	// 1. Health check (no auth)
	resp, _ := client.Get(ts.URL + "/api/v1/health")
	if resp.StatusCode != 200 {
		t.Fatalf("health: %d", resp.StatusCode)
	}

	// 2. Create device (with auth)
	body := `{"name":"test-laptop","public_key":"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestKey test@laptop","accepts_ssh":false,"tags":["test"]}`
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/devices", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-api-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ = client.Do(req)
	if resp.StatusCode != 201 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("create device: %d %s", resp.StatusCode, string(b))
	}

	var device map[string]any
	json.NewDecoder(resp.Body).Decode(&device)
	deviceID := device["id"].(string)

	// 3. Create second device (server)
	body2 := `{"name":"test-server","public_key":"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIServerKey root@server","accepts_ssh":true,"tags":["linux"]}`
	req, _ = http.NewRequest("POST", ts.URL+"/api/v1/devices", strings.NewReader(body2))
	req.Header.Set("Authorization", "Bearer test-api-key")
	req.Header.Set("Content-Type", "application/json")
	resp, _ = client.Do(req)
	if resp.StatusCode != 201 {
		t.Fatalf("create server device: %d", resp.StatusCode)
	}

	// 4. List devices (with auth)
	req, _ = http.NewRequest("GET", ts.URL+"/api/v1/devices", nil)
	req.Header.Set("Authorization", "Bearer test-api-key")
	resp, _ = client.Do(req)
	var devices []map[string]any
	json.NewDecoder(resp.Body).Decode(&devices)
	if len(devices) != 2 {
		t.Errorf("list: got %d devices, want 2", len(devices))
	}

	// 5. Get authorized_keys (no auth)
	resp, _ = client.Get(ts.URL + "/api/v1/authorized_keys")
	keysBody, _ := io.ReadAll(resp.Body)
	keys := string(keysBody)
	if !strings.Contains(keys, "TestKey") || !strings.Contains(keys, "ServerKey") {
		t.Errorf("authorized_keys missing keys: %s", keys)
	}

	// 6. Revoke device
	req, _ = http.NewRequest("POST", ts.URL+"/api/v1/devices/"+deviceID+"/revoke", nil)
	req.Header.Set("Authorization", "Bearer test-api-key")
	resp, _ = client.Do(req)
	if resp.StatusCode != 200 {
		t.Fatalf("revoke: %d", resp.StatusCode)
	}

	// 7. Authorized keys should only have server key now
	resp, _ = client.Get(ts.URL + "/api/v1/authorized_keys")
	keysBody, _ = io.ReadAll(resp.Body)
	keys = string(keysBody)
	if strings.Contains(keys, "TestKey") {
		t.Error("revoked key should not appear in authorized_keys")
	}
	if !strings.Contains(keys, "ServerKey") {
		t.Error("active key missing from authorized_keys")
	}

	// 8. Unauthorized access should fail
	req, _ = http.NewRequest("GET", ts.URL+"/api/v1/devices", nil)
	resp, _ = client.Do(req)
	if resp.StatusCode != 401 {
		t.Errorf("unauthed list: got %d, want 401", resp.StatusCode)
	}

	// 9. Web UI pages should load
	resp, _ = client.Get(ts.URL + "/")
	if resp.StatusCode != 200 {
		t.Errorf("web root: %d", resp.StatusCode)
	}
	resp, _ = client.Get(ts.URL + "/authorized-keys")
	if resp.StatusCode != 200 {
		t.Errorf("web authorized-keys: %d", resp.StatusCode)
	}
}
```

**Step 2: Run integration test**

Run: `cd /Users/driversti/Projects/keyforge && go test ./test/ -v`
Expected: PASS

**Step 3: Run all tests**

Run: `cd /Users/driversti/Projects/keyforge && go test ./... -v`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add -A
git commit -m "test: integration smoke test covering full device lifecycle"
```

---

### Task 14: Final Wiring & gitignore

**Files:**
- Create: `.gitignore`
- Verify: full `cmd/keyforge/main.go` has all commands registered

**Step 1: Create .gitignore**

Create `.gitignore`:
```
bin/
keyforge-data/
*.db
*.db-wal
*.db-shm
.DS_Store
```

**Step 2: Final build + all tests**

Run: `cd /Users/driversti/Projects/keyforge && make build && go test ./... -v`
Expected: Build succeeds, ALL tests pass.

**Step 3: Commit**

```bash
git add -A
git commit -m "chore: add gitignore and finalize Phase 1 MVP"
```

---

## Summary

After completing all 14 tasks, you'll have a working KeyForge MVP with:
- **SQLite database** with auto-migration (devices, tokens, audit_log, settings)
- **REST API**: CRUD for devices, `/authorized_keys` plaintext endpoint, health check
- **API key auth**: auto-generated on first run, Bearer token validation
- **CLI**: `serve`, `device add/list/revoke/reactivate/delete`, `keys`
- **Web UI**: device list with actions, add device form, authorized keys copy page
- **Integration test**: full lifecycle coverage

The binary is ~15MB, zero external dependencies at runtime, cross-compilable to any platform.
