package keys

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

// GenerateED25519Key generates an ed25519 SSH key pair at the given path.
// Private key goes to `path`, public key to `path.pub`.
// If files already exist, returns nil without overwriting.
func GenerateED25519Key(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil // key already exists
	}

	// Ensure directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create key directory: %w", err)
	}

	// Generate key pair.
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate ed25519 key: %w", err)
	}

	// Marshal private key to OpenSSH PEM format.
	privBlock, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return fmt.Errorf("marshal private key: %w", err)
	}
	privPEM := pem.EncodeToMemory(privBlock)

	if err := os.WriteFile(path, privPEM, 0o600); err != nil {
		return fmt.Errorf("write private key: %w", err)
	}

	// Marshal public key to authorized_keys format.
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return fmt.Errorf("convert public key: %w", err)
	}
	pubBytes := ssh.MarshalAuthorizedKey(sshPub)

	if err := os.WriteFile(path+".pub", pubBytes, 0o644); err != nil {
		return fmt.Errorf("write public key: %w", err)
	}

	return nil
}

// ReadPublicKey reads and returns the SSH public key from a .pub file.
func ReadPublicKey(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read public key file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// DefaultKeyPath returns ~/.ssh/id_ed25519.
func DefaultKeyPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "id_ed25519")
}
