package db

import (
	"testing"
)

func TestLogAuditEntry(t *testing.T) {
	db := newTestDB(t)

	deviceID := "test-device-id"
	err := db.LogAudit("device.created", &deviceID, "registered new device", "192.168.1.1")
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

	entry := entries[0]
	if entry.Action != "device.created" {
		t.Errorf("expected action 'device.created', got %q", entry.Action)
	}
	if entry.DeviceID == nil || *entry.DeviceID != deviceID {
		t.Errorf("expected device_id %q, got %v", deviceID, entry.DeviceID)
	}
	if entry.Details != "registered new device" {
		t.Errorf("expected details 'registered new device', got %q", entry.Details)
	}
	if entry.SourceIP != "192.168.1.1" {
		t.Errorf("expected source_ip '192.168.1.1', got %q", entry.SourceIP)
	}
}
