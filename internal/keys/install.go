package keys

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	ManagedHeader = "# --- KeyForge Managed Keys (DO NOT EDIT) ---"
	ManagedFooter = "# --- End KeyForge Managed Keys ---"
)

// InstallKeys writes the given keys content into the authorized_keys file,
// using managed section markers. Content outside the markers is preserved.
func InstallKeys(keysContent string, authorizedKeysPath string) error {
	managedSection := ManagedHeader + "\n" + keysContent + "\n" + ManagedFooter + "\n"

	// Ensure parent directory exists.
	dir := filepath.Dir(authorizedKeysPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	existing, err := os.ReadFile(authorizedKeysPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist — create with just the managed section.
			return os.WriteFile(authorizedKeysPath, []byte(managedSection), 0o600)
		}
		return fmt.Errorf("read %s: %w", authorizedKeysPath, err)
	}

	content := string(existing)

	headerIdx := strings.Index(content, ManagedHeader)
	footerIdx := strings.Index(content, ManagedFooter)

	if headerIdx >= 0 && footerIdx >= 0 {
		// Replace existing managed section (inclusive of markers).
		before := content[:headerIdx]
		after := content[footerIdx+len(ManagedFooter):]
		// Strip the trailing newline after footer if present.
		if strings.HasPrefix(after, "\n") {
			after = after[1:]
		}
		newContent := before + managedSection + after
		return os.WriteFile(authorizedKeysPath, []byte(newContent), 0o600)
	}

	// File exists but has no managed section — append.
	separator := "\n"
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		separator = "\n\n"
	} else if len(content) > 0 {
		separator = "\n"
	}

	newContent := content + separator + managedSection
	return os.WriteFile(authorizedKeysPath, []byte(newContent), 0o600)
}
