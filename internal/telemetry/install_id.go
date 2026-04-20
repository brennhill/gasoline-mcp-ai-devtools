// install_id.go — Stable install ID for anonymous session correlation.
// Writes are atomic (temp + Sync + Rename) so a crash mid-write cannot leave
// a zero-length file. If the id file is missing/corrupt, loadOrGenerateInstallID
// first tries a deterministic HMAC derivation from (machine_id, uid, hostname)
// so recoverable environments get the same ID back instead of a fresh random.

package telemetry

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// kaboomDir is the directory where install_id is persisted. Overridable for tests.
var kaboomDir = defaultKaboomDir()

// cachedInstallID holds the in-memory cached value after first load.
var cachedInstallID string

// installIDOnce ensures the install ID is loaded/generated exactly once.
var installIDOnce sync.Once

// firstToolCallMu protects first-tool-call state across goroutines.
var firstToolCallMu sync.Mutex

// firstToolCallOnce ensures the persisted state is loaded once per process.
var firstToolCallOnce sync.Once

// cachedFirstToolCallInstallID is the install ID that has already emitted first_tool_call.
var cachedFirstToolCallInstallID string

// readMachineID is a package-level indirection so tests can stub the OS lookup.
var readMachineID = readMachineIDFromOS

func defaultKaboomDir() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(home, ".kaboom")
}

// Warm pre-loads install ID and session state so the first tool call
// doesn't incur filesystem I/O on the hot path. Call at daemon startup.
func Warm() {
	GetInstallID()
	TouchSession()
}

// GetInstallID returns the persistent anonymous install ID.
// On first call, reads from ~/.kaboom/install_id or generates a new 12-char hex string.
// Thread-safe via sync.Once. Returns empty string if a stable install ID cannot be read or persisted.
func GetInstallID() string {
	installIDOnce.Do(func() {
		cachedInstallID = loadOrGenerateInstallID()
	})
	return cachedInstallID
}

func loadOrGenerateInstallID() string {
	if strings.TrimSpace(kaboomDir) == "" {
		return ""
	}

	idPath := filepath.Join(kaboomDir, "install_id")

	// Try to read existing file.
	data, err := os.ReadFile(idPath)
	if err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id
		}
		// Whitespace-only content: fall through to derive/random self-heal path.
	} else if !os.IsNotExist(err) {
		return ""
	}

	// Ensure dir exists before any persist attempt.
	if err := os.MkdirAll(kaboomDir, 0o700); err != nil {
		return ""
	}

	// Prefer deterministic derivation so wiped/restored environments recover
	// the same ID instead of appearing as a new install.
	if derived, ok := deriveInstallID(); ok {
		if err := writeInstallIDAtomic(idPath, derived); err != nil {
			return derived
		}
		return derived
	}

	// Fall back to a fresh random ID.
	id := generateRandomID()
	if err := writeInstallIDAtomic(idPath, id); err != nil {
		return id
	}
	return id
}

// writeInstallIDAtomic writes id to idPath atomically: temp file in the same
// directory, Sync, Close, Rename. On any error the temp file is removed so
// we never leak partially-written state.
func writeInstallIDAtomic(idPath, id string) (retErr error) {
	dir := filepath.Dir(idPath)
	tmp, err := os.CreateTemp(dir, "install_id.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = os.Remove(tmpName)
	}
	defer func() {
		if retErr != nil {
			cleanup()
		}
	}()

	if _, err := tmp.Write([]byte(id)); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmpName, idPath); err != nil {
		return err
	}
	return nil
}

// deriveInstallID attempts to derive a stable 12-hex install id from
// (machine_id, user uid, hostname). Returns ("", false) if any primitive
// is unavailable so the caller falls back to random generation.
func deriveInstallID() (string, bool) {
	machineID, ok := readMachineID()
	if !ok {
		return "", false
	}
	machineID = strings.TrimSpace(machineID)
	if machineID == "" {
		return "", false
	}

	u, err := user.Current()
	if err != nil || strings.TrimSpace(u.Uid) == "" {
		return "", false
	}
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		return "", false
	}

	return deriveInstallIDFromParts(machineID, u.Uid, hostname), true
}

// deriveInstallIDFromParts is the pure HMAC derivation, exposed for tests.
// Output is the first 6 bytes of HMAC-SHA256(machineID, "kaboom-install-id-v1:uid:hostname")
// hex-encoded (12 lowercase hex chars).
func deriveInstallIDFromParts(machineID, uid, hostname string) string {
	mac := hmac.New(sha256.New, []byte(machineID))
	_, _ = io.WriteString(mac, "kaboom-install-id-v1:")
	_, _ = io.WriteString(mac, uid)
	_, _ = io.WriteString(mac, ":")
	_, _ = io.WriteString(mac, hostname)
	sum := mac.Sum(nil)
	return hex.EncodeToString(sum[:6])
}

// readMachineIDFromOS returns a stable per-host identifier using OS primitives.
// Returns ("", false) on any failure so callers can fall back.
func readMachineIDFromOS() (string, bool) {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
		if err != nil {
			return "", false
		}
		return parseIORegIOPlatformUUID(string(out))
	case "linux":
		if data, err := os.ReadFile("/etc/machine-id"); err == nil {
			if id := strings.TrimSpace(string(data)); id != "" {
				return id, true
			}
		}
		if data, err := os.ReadFile("/var/lib/dbus/machine-id"); err == nil {
			if id := strings.TrimSpace(string(data)); id != "" {
				return id, true
			}
		}
		return "", false
	case "windows":
		out, err := exec.Command("reg", "query",
			`HKLM\SOFTWARE\Microsoft\Cryptography`, "/v", "MachineGuid").Output()
		if err != nil {
			return "", false
		}
		return parseWindowsMachineGUID(string(out))
	default:
		return "", false
	}
}

// parseIORegIOPlatformUUID extracts the UUID from an `ioreg -rd1 -c IOPlatformExpertDevice` block.
// Expected line: `    "IOPlatformUUID" = "D5C17F5C-41A1-546A-A49B-6280B33B1419"`
func parseIORegIOPlatformUUID(out string) (string, bool) {
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "IOPlatformUUID") {
			continue
		}
		// Pull the quoted value after `=`.
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		rest := line[eq+1:]
		first := strings.IndexByte(rest, '"')
		if first < 0 {
			continue
		}
		second := strings.IndexByte(rest[first+1:], '"')
		if second < 0 {
			continue
		}
		id := strings.TrimSpace(rest[first+1 : first+1+second])
		if id != "" {
			return id, true
		}
	}
	return "", false
}

// parseWindowsMachineGUID extracts the GUID from `reg query` output.
// Expected line: `    MachineGuid    REG_SZ    <uuid>`
func parseWindowsMachineGUID(out string) (string, bool) {
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "MachineGuid") {
			continue
		}
		idx := strings.Index(line, "REG_SZ")
		if idx < 0 {
			continue
		}
		id := strings.TrimSpace(line[idx+len("REG_SZ"):])
		if id != "" {
			return id, true
		}
	}
	return "", false
}

func loadFirstToolCallInstallID() string {
	if strings.TrimSpace(kaboomDir) == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(kaboomDir, "first_tool_call_install_id"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// markFirstToolCallEmittedForInstall persists first-tool state and returns true
// only the first time it is called for the current install ID.
func markFirstToolCallEmittedForInstall() bool {
	firstToolCallMu.Lock()
	defer firstToolCallMu.Unlock()

	firstToolCallOnce.Do(func() {
		cachedFirstToolCallInstallID = loadFirstToolCallInstallID()
	})

	installID := GetInstallID()
	if installID == "" {
		return false
	}
	if cachedFirstToolCallInstallID == installID {
		return false
	}

	if err := os.MkdirAll(kaboomDir, 0o700); err != nil {
		return false
	}
	if err := os.WriteFile(
		filepath.Join(kaboomDir, "first_tool_call_install_id"),
		[]byte(installID),
		0o600,
	); err != nil {
		return false
	}
	cachedFirstToolCallInstallID = installID

	return true
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

// resetFirstToolCallState clears the cached first-tool-call state for testing.
func resetFirstToolCallState() {
	firstToolCallOnce = sync.Once{}
	cachedFirstToolCallInstallID = ""
}
