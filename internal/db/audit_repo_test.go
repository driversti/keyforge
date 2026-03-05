package db

import (
	"fmt"
	"testing"
)

func TestLogAudit(t *testing.T) {
	db := newTestDB(t)

	devID := "dev-123"
	err := db.LogAudit("device.created", &devID, "registered device", "192.168.1.1")
	if err != nil {
		t.Fatalf("LogAudit: %v", err)
	}

	entries, err := db.ListAudit(10, 0)
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Action != "device.created" {
		t.Errorf("expected action 'device.created', got %q", e.Action)
	}
	if e.DeviceID == nil || *e.DeviceID != "dev-123" {
		t.Errorf("expected device_id 'dev-123', got %v", e.DeviceID)
	}
	if e.Details != "registered device" {
		t.Errorf("expected details 'registered device', got %q", e.Details)
	}
	if e.SourceIP != "192.168.1.1" {
		t.Errorf("expected source_ip '192.168.1.1', got %q", e.SourceIP)
	}
}

func TestListAudit_Pagination(t *testing.T) {
	db := newTestDB(t)

	for i := 0; i < 5; i++ {
		err := db.LogAudit("action", nil, fmt.Sprintf("entry-%d", i), "127.0.0.1")
		if err != nil {
			t.Fatalf("LogAudit(%d): %v", i, err)
		}
	}

	// First page: limit=2, offset=0
	page1, err := db.ListAudit(2, 0)
	if err != nil {
		t.Fatalf("ListAudit page1: %v", err)
	}
	if len(page1) != 2 {
		t.Errorf("expected 2 entries on page1, got %d", len(page1))
	}

	// Second page: limit=2, offset=2
	page2, err := db.ListAudit(2, 2)
	if err != nil {
		t.Fatalf("ListAudit page2: %v", err)
	}
	if len(page2) != 2 {
		t.Errorf("expected 2 entries on page2, got %d", len(page2))
	}

	// Verify pages don't overlap (different IDs).
	if page1[0].ID == page2[0].ID {
		t.Error("page1 and page2 returned the same first entry")
	}
}

func TestCountAudit(t *testing.T) {
	db := newTestDB(t)

	for i := 0; i < 3; i++ {
		err := db.LogAudit("action", nil, fmt.Sprintf("entry-%d", i), "127.0.0.1")
		if err != nil {
			t.Fatalf("LogAudit(%d): %v", i, err)
		}
	}

	count, err := db.CountAudit()
	if err != nil {
		t.Fatalf("CountAudit: %v", err)
	}
	if count != 3 {
		t.Errorf("expected count 3, got %d", count)
	}
}

func TestListAudit_Empty(t *testing.T) {
	db := newTestDB(t)

	entries, err := db.ListAudit(10, 0)
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil slice from empty table, got %v", entries)
	}
}
