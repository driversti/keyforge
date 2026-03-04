package db

import (
	"testing"

	"github.com/driversti/keyforge/internal/models"
)

// newTestDB creates an in-memory SQLite database for testing.
func newTestDB(t *testing.T) *DB {
	t.Helper()
	d, err := New(":memory:")
	if err != nil {
		t.Fatalf("newTestDB: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestCreateDevice(t *testing.T) {
	db := newTestDB(t)

	req := models.CreateDeviceRequest{
		Name:      "my-laptop",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest user@host",
		Tags:      []string{"dev", "personal"},
	}

	device, err := db.CreateDevice(req)
	if err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}

	if device.Name != "my-laptop" {
		t.Errorf("expected name 'my-laptop', got %q", device.Name)
	}
	if device.Status != models.StatusActive {
		t.Errorf("expected status 'active', got %q", device.Status)
	}
	if device.Fingerprint == "" {
		t.Error("expected fingerprint to be non-empty")
	}
	if device.ID == "" {
		t.Error("expected ID to be non-empty")
	}
}

func TestCreateDevice_DuplicateName(t *testing.T) {
	db := newTestDB(t)

	req := models.CreateDeviceRequest{
		Name:      "my-laptop",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest user@host",
	}

	_, err := db.CreateDevice(req)
	if err != nil {
		t.Fatalf("first CreateDevice: %v", err)
	}

	_, err = db.CreateDevice(req)
	if err == nil {
		t.Fatal("expected error for duplicate name, got nil")
	}
}

func TestListDevices(t *testing.T) {
	db := newTestDB(t)

	for _, name := range []string{"device-1", "device-2"} {
		_, err := db.CreateDevice(models.CreateDeviceRequest{
			Name:      name,
			PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest user@host",
		})
		if err != nil {
			t.Fatalf("CreateDevice(%s): %v", name, err)
		}
	}

	devices, err := db.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if len(devices) != 2 {
		t.Errorf("expected 2 devices, got %d", len(devices))
	}
}

func TestGetDevice(t *testing.T) {
	db := newTestDB(t)

	created, err := db.CreateDevice(models.CreateDeviceRequest{
		Name:      "my-laptop",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest user@host",
	})
	if err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}

	got, err := db.GetDevice(created.ID)
	if err != nil {
		t.Fatalf("GetDevice: %v", err)
	}
	if got.Name != "my-laptop" {
		t.Errorf("expected name 'my-laptop', got %q", got.Name)
	}
}

func TestRevokeDevice(t *testing.T) {
	db := newTestDB(t)

	created, err := db.CreateDevice(models.CreateDeviceRequest{
		Name:      "my-laptop",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest user@host",
	})
	if err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}

	if err := db.RevokeDevice(created.ID); err != nil {
		t.Fatalf("RevokeDevice: %v", err)
	}

	got, err := db.GetDevice(created.ID)
	if err != nil {
		t.Fatalf("GetDevice after revoke: %v", err)
	}
	if got.Status != models.StatusRevoked {
		t.Errorf("expected status 'revoked', got %q", got.Status)
	}
}

func TestDeleteDevice(t *testing.T) {
	db := newTestDB(t)

	created, err := db.CreateDevice(models.CreateDeviceRequest{
		Name:      "my-laptop",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest user@host",
	})
	if err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}

	if err := db.DeleteDevice(created.ID); err != nil {
		t.Fatalf("DeleteDevice: %v", err)
	}

	_, err = db.GetDevice(created.ID)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestGetActivePublicKeys(t *testing.T) {
	db := newTestDB(t)

	d1, err := db.CreateDevice(models.CreateDeviceRequest{
		Name:      "device-1",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest1 user@host",
	})
	if err != nil {
		t.Fatalf("CreateDevice(device-1): %v", err)
	}

	_, err = db.CreateDevice(models.CreateDeviceRequest{
		Name:      "device-2",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest2 user@host",
	})
	if err != nil {
		t.Fatalf("CreateDevice(device-2): %v", err)
	}

	if err := db.RevokeDevice(d1.ID); err != nil {
		t.Fatalf("RevokeDevice: %v", err)
	}

	active, err := db.GetActivePublicKeys()
	if err != nil {
		t.Fatalf("GetActivePublicKeys: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("expected 1 active device, got %d", len(active))
	}
}

func TestUpdateDevice(t *testing.T) {
	db := newTestDB(t)

	created, err := db.CreateDevice(models.CreateDeviceRequest{
		Name:       "my-laptop",
		PublicKey:  "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest user@host",
		AcceptsSSH: false,
		Tags:       []string{"dev"},
	})
	if err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}

	newName := "my-desktop"
	newSSH := true
	newTags := []string{"prod", "server"}

	err = db.UpdateDevice(created.ID, models.UpdateDeviceRequest{
		Name:       &newName,
		AcceptsSSH: &newSSH,
		Tags:       newTags,
	})
	if err != nil {
		t.Fatalf("UpdateDevice: %v", err)
	}

	got, err := db.GetDevice(created.ID)
	if err != nil {
		t.Fatalf("GetDevice after update: %v", err)
	}

	if got.Name != "my-desktop" {
		t.Errorf("expected name 'my-desktop', got %q", got.Name)
	}
	if !got.AcceptsSSH {
		t.Error("expected accepts_ssh to be true")
	}
	if len(got.Tags) != 2 || got.Tags[0] != "prod" || got.Tags[1] != "server" {
		t.Errorf("expected tags [prod server], got %v", got.Tags)
	}
}
