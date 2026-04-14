// install_id.go — Random install ID for anonymous session correlation.

package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// kaboomDir is the directory where install_id is persisted. Overridable for tests.
var kaboomDir = defaultKaboomDir()

// cachedInstallID holds the in-memory cached value after first load.
var cachedInstallID string

// installIDOnce ensures the install ID is loaded/generated exactly once.
var installIDOnce sync.Once

func defaultKaboomDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), ".kaboom")
	}
	return filepath.Join(home, ".kaboom")
}

// GetInstallID returns the persistent anonymous install ID.
// On first call, reads from ~/.kaboom/install_id or generates a new 12-char hex string.
// Thread-safe via sync.Once. Never returns an error — falls back to a fresh random ID.
func GetInstallID() string {
	installIDOnce.Do(func() {
		cachedInstallID = loadOrGenerateInstallID()
	})
	return cachedInstallID
}

func loadOrGenerateInstallID() string {
	idPath := filepath.Join(kaboomDir, "install_id")

	// Try to read existing file.
	data, err := os.ReadFile(idPath)
	if err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id
		}
	}

	// Generate a new random ID.
	id := generateRandomID()

	// Best-effort persist: create dir and write file.
	if err := os.MkdirAll(kaboomDir, 0700); err == nil {
		_ = os.WriteFile(idPath, []byte(id), 0600)
	}

	return id
}

func generateRandomID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		// Fallback: return a zero-filled ID rather than failing.
		return "000000000000"
	}
	return hex.EncodeToString(b)
}

// overrideKaboomDir sets a custom directory for testing.
func overrideKaboomDir(dir string) {
	kaboomDir = dir
}

// resetKaboomDir restores the default Kaboom directory after testing.
func resetKaboomDir() {
	kaboomDir = defaultKaboomDir()
}

// resetInstallIDState clears the cached install ID and sync.Once for testing.
func resetInstallIDState() {
	installIDOnce = sync.Once{}
	cachedInstallID = ""
}
