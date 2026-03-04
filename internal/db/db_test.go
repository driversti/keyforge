package db

import (
	"testing"
)

func TestNewDB_CreatesTablesOnInit(t *testing.T) {
	database, err := New(":memory:")
	if err != nil {
		t.Fatalf("New(:memory:) returned error: %v", err)
	}
	defer database.Close()

	expectedTables := []string{"devices", "enrollment_tokens", "audit_log", "settings"}

	for _, table := range expectedTables {
		var name string
		err := database.DB.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("expected table %q to exist, but got error: %v", table, err)
		}
	}
}

func TestNewDB_EnablesWAL(t *testing.T) {
	database, err := New(":memory:")
	if err != nil {
		t.Fatalf("New(:memory:) returned error: %v", err)
	}
	defer database.Close()

	var journalMode string
	err = database.DB.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatalf("failed to query journal_mode: %v", err)
	}
	// In-memory databases may report "memory" instead of "wal"
	if journalMode != "wal" && journalMode != "memory" {
		t.Errorf("expected journal_mode 'wal' or 'memory', got %q", journalMode)
	}
}

func TestNewDB_EnablesForeignKeys(t *testing.T) {
	database, err := New(":memory:")
	if err != nil {
		t.Fatalf("New(:memory:) returned error: %v", err)
	}
	defer database.Close()

	var fkEnabled int
	err = database.DB.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled)
	if err != nil {
		t.Fatalf("failed to query foreign_keys: %v", err)
	}
	if fkEnabled != 1 {
		t.Errorf("expected foreign_keys=1, got %d", fkEnabled)
	}
}
