// install_id.go — Random install ID for anonymous session correlation.

package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
)

// strumDir is the directory where install_id is persisted. Overridable for tests.
var strumDir = defaultStrumDir()

// cachedInstallID holds the in-memory cached value after first load.
var cachedInstallID string

// installIDOnce ensures the install ID is loaded/generated exactly once.
var installIDOnce sync.Once

func defaultStrumDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), ".strum")
	}
	return filepath.Join(home, ".strum")
}

// GetInstallID returns the persistent anonymous install ID.
// On first call, reads from ~/.strum/install_id or generates a new 12-char hex string.
// Thread-safe via sync.Once. Never returns an error — falls back to a fresh random ID.
func GetInstallID() string {
	installIDOnce.Do(func() {
		cachedInstallID = loadOrGenerateInstallID()
	})
	return cachedInstallID
}

func loadOrGenerateInstallID() string {
	idPath := filepath.Join(strumDir, "install_id")

	// Try to read existing file.
	data, err := os.ReadFile(idPath)
	if err == nil && len(data) > 0 {
		return string(data)
	}

	// Generate a new random ID.
	id := generateRandomID()

	// Best-effort persist: create dir and write file.
	if err := os.MkdirAll(strumDir, 0700); err == nil {
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

// overrideStrumDir sets a custom directory for testing.
func overrideStrumDir(dir string) {
	strumDir = dir
}

// resetStrumDir restores the default strum directory after testing.
func resetStrumDir() {
	strumDir = defaultStrumDir()
}

// resetInstallIDState clears the cached install ID and sync.Once for testing.
func resetInstallIDState() {
	installIDOnce = sync.Once{}
	cachedInstallID = ""
}
