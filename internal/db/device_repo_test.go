package db

import (
	"strings"
	"testing"

	"driversti.dev/keyforge/internal/models"
)

// Valid SSH test keys for use in tests.
const (
	testKey1 = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFXQEx7dJI2DHGuq5nQzd0yozL4XHRRSdlaokZYy0ipS test@host"
	testKey2 = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICKyQHFUMsbDtH66sAAI35pIsLxLCfCUc29crMc0/KHn test2@host"
	testKey3 = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOcfZQ4Suj5TMETCp7FACYFuO/6RiPj3wP6Wjm3Uwokf test3@host"
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
		PublicKey: testKey1,
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
	if !strings.HasPrefix(device.Fingerprint, "SHA256:") {
		t.Errorf("expected fingerprint to start with 'SHA256:', got %q", device.Fingerprint)
	}
	if device.ID == "" {
		t.Error("expected ID to be non-empty")
	}
}

func TestCreateDevice_InvalidKey(t *testing.T) {
	db := newTestDB(t)

	req := models.CreateDeviceRequest{
		Name:      "my-laptop",
		PublicKey: "not-a-valid-ssh-key",
	}

	_, err := db.CreateDevice(req)
	if err == nil {
		t.Fatal("expected error for invalid SSH key, got nil")
	}
	if !strings.Contains(err.Error(), "invalid SSH public key") {
		t.Errorf("expected 'invalid SSH public key' error, got: %v", err)
	}
}

func TestCreateDevice_DuplicatePublicKey(t *testing.T) {
	db := newTestDB(t)

	_, err := db.CreateDevice(models.CreateDeviceRequest{
		Name:      "device-1",
		PublicKey: testKey1,
	})
	if err != nil {
		t.Fatalf("first CreateDevice: %v", err)
	}

	_, err = db.CreateDevice(models.CreateDeviceRequest{
		Name:      "device-2",
		PublicKey: testKey1,
	})
	if err == nil {
		t.Fatal("expected error for duplicate public key, got nil")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("expected 'already registered' error, got: %v", err)
	}
}

func TestCreateDevice_DuplicateName(t *testing.T) {
	db := newTestDB(t)

	_, err := db.CreateDevice(models.CreateDeviceRequest{
		Name:      "my-laptop",
		PublicKey: testKey1,
	})
	if err != nil {
		t.Fatalf("first CreateDevice: %v", err)
	}

	_, err = db.CreateDevice(models.CreateDeviceRequest{
		Name:      "my-laptop",
		PublicKey: testKey2,
	})
	if err == nil {
		t.Fatal("expected error for duplicate name, got nil")
	}
}

func TestListDevices(t *testing.T) {
	db := newTestDB(t)

	keys := []string{testKey1, testKey2}
	for i, name := range []string{"device-1", "device-2"} {
		_, err := db.CreateDevice(models.CreateDeviceRequest{
			Name:      name,
			PublicKey: keys[i],
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
		PublicKey: testKey1,
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
		PublicKey: testKey1,
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
		PublicKey: testKey1,
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
		PublicKey: testKey1,
	})
	if err != nil {
		t.Fatalf("CreateDevice(device-1): %v", err)
	}

	_, err = db.CreateDevice(models.CreateDeviceRequest{
		Name:      "device-2",
		PublicKey: testKey2,
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
		PublicKey:  testKey1,
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

func TestSearchDevices_ByName(t *testing.T) {
	db := newTestDB(t)

	names := []string{"alpha-laptop", "beta-server", "gamma-laptop"}
	keys := []string{testKey1, testKey2, testKey3}
	for i, name := range names {
		_, err := db.CreateDevice(models.CreateDeviceRequest{
			Name:      name,
			PublicKey: keys[i],
		})
		if err != nil {
			t.Fatalf("CreateDevice(%s): %v", name, err)
		}
	}

	devices, err := db.SearchDevices("laptop", "")
	if err != nil {
		t.Fatalf("SearchDevices: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices matching 'laptop', got %d", len(devices))
	}

	for _, d := range devices {
		if !strings.Contains(d.Name, "laptop") {
			t.Errorf("expected name containing 'laptop', got %q", d.Name)
		}
	}
}

func TestSearchDevices_ByStatus(t *testing.T) {
	db := newTestDB(t)

	d1, err := db.CreateDevice(models.CreateDeviceRequest{
		Name:      "device-a",
		PublicKey: testKey1,
	})
	if err != nil {
		t.Fatalf("CreateDevice(device-a): %v", err)
	}

	_, err = db.CreateDevice(models.CreateDeviceRequest{
		Name:      "device-b",
		PublicKey: testKey2,
	})
	if err != nil {
		t.Fatalf("CreateDevice(device-b): %v", err)
	}

	if err := db.RevokeDevice(d1.ID); err != nil {
		t.Fatalf("RevokeDevice: %v", err)
	}

	active, err := db.SearchDevices("", "active")
	if err != nil {
		t.Fatalf("SearchDevices(active): %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active device, got %d", len(active))
	}
	if active[0].Name != "device-b" {
		t.Errorf("expected 'device-b', got %q", active[0].Name)
	}

	revoked, err := db.SearchDevices("", "revoked")
	if err != nil {
		t.Fatalf("SearchDevices(revoked): %v", err)
	}
	if len(revoked) != 1 {
		t.Fatalf("expected 1 revoked device, got %d", len(revoked))
	}
	if revoked[0].Name != "device-a" {
		t.Errorf("expected 'device-a', got %q", revoked[0].Name)
	}
}

func TestSearchDevices_NoFilter(t *testing.T) {
	db := newTestDB(t)

	keys := []string{testKey1, testKey2}
	for i, name := range []string{"device-x", "device-y"} {
		_, err := db.CreateDevice(models.CreateDeviceRequest{
			Name:      name,
			PublicKey: keys[i],
		})
		if err != nil {
			t.Fatalf("CreateDevice(%s): %v", name, err)
		}
	}

	devices, err := db.SearchDevices("", "")
	if err != nil {
		t.Fatalf("SearchDevices: %v", err)
	}
	if len(devices) != 2 {
		t.Errorf("expected 2 devices with no filter, got %d", len(devices))
	}
}
