// install_id.go — Stable install ID for anonymous session correlation.
//
// Hardening layers (in priority order on read):
//   1. Format validation — only `^[0-9a-f]{12}$` is accepted as a valid ID.
//      A corrupted file (zero-length, truncated, non-hex) is treated as
//      missing and falls through to backup/secondary/derive.
//   2. Backup file — every successful write also writes `install_id.bak`
//      next to the primary so a single accidental delete is recoverable.
//   3. Secondary location — a platform-stable mirror outside `~/.kaboom/`
//      (Application Support / XDG_STATE_HOME / %LOCALAPPDATA%) survives
//      home-dir migrations and rsync exclusions.
//   4. Drift detection — when a stored ID differs from the deterministic
//      derivation (e.g., hostname rename), the registered drift logger
//      fires once. Stored value always wins; drift is observability only.
//
// Writes remain atomic (temp + Sync + Rename) so a crash mid-write cannot
// leave a zero-length file. If all four locations are missing/corrupt,
// loadOrGenerateInstallID tries a deterministic HMAC derivation from
// (machine_id, uid, hostname) so recoverable environments get the same ID
// back instead of a fresh random.

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
	"sync/atomic"
	"time"
)

// kaboomDir is the directory where install_id is persisted. Overridable for tests.
var kaboomDir = defaultKaboomDir()

// cachedInstallIDPtr holds the in-memory cached value after first load.
// atomic.Pointer + installIDLoadMu replace the prior sync.Once pattern so
// resetInstallIDState (test-only) is race-safe under -race -count=N even
// when prior-test goroutines are still mid-load.
var cachedInstallIDPtr atomic.Pointer[string]

// installIDLoadMu serializes the install-ID load path so concurrent first
// callers see exactly one loadOrGenerateInstallID invocation, and tests can
// reset state safely while a load is in flight.
var installIDLoadMu sync.Mutex

// firstToolCallMu protects first-tool-call state across goroutines.
var firstToolCallMu sync.Mutex

// cachedFirstToolCallLoaded gates the persisted-state load so reset can
// re-arm it without racing sync.Once replacement.
var cachedFirstToolCallLoaded bool

// cachedFirstToolCallInstallID is the install ID that has already emitted first_tool_call.
var cachedFirstToolCallInstallID string

// readMachineID is a package-level indirection so tests can stub the OS lookup.
var readMachineID = readMachineIDFromOS

// secondaryDirOverride lets tests redirect the platform-stable mirror.
// Empty means "compute from runtime.GOOS + UserHomeDir". A non-empty value
// is used directly. The mirror is disabled entirely when secondaryDirDisabled
// is set (by overrideKaboomDir for tests written against single-location
// semantics).
var secondaryDirOverride string

// secondaryDirDisabled disables the cross-location mirror regardless of
// secondaryDirOverride. Set by overrideKaboomDir for back-compat with tests
// that pre-date the four-location feature.
var secondaryDirDisabled bool

// Drift detection (CheckInstallIDDrift, SetInstallIDDriftLogFn, lineage
// persistence) lives in install_id_drift.go.

// Test hooks block just before persistence so concurrency tests can force races.
var installIDBeforePersistHook func()
var firstToolCallBeforePersistHook func()

const (
	installStateLockTimeout = 2 * time.Second
	installStateLockPoll    = 10 * time.Millisecond
	installStateLockStale   = 10 * time.Second
)

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
// Thread-safe: lock-free fast path via atomic.Pointer; the slow path serializes
// concurrent first-load callers behind installIDLoadMu. Returns empty string
// if a stable install ID cannot be read or persisted.
func GetInstallID() string {
	if p := cachedInstallIDPtr.Load(); p != nil {
		return *p
	}
	installIDLoadMu.Lock()
	defer installIDLoadMu.Unlock()
	if p := cachedInstallIDPtr.Load(); p != nil {
		return *p
	}
	id := loadOrGenerateInstallID()
	cachedInstallIDPtr.Store(&id)
	return id
}

func loadOrGenerateInstallID() string {
	if strings.TrimSpace(kaboomDir) == "" {
		return ""
	}

	locations := installIDLocations()

	// Fast path: try every known location without taking the lock.
	// First valid (12-char lowercase hex) win. Drift detection runs OUT
	// of this function (CheckInstallIDDrift, called post-Warm) to avoid
	// re-entering GetInstallID's sync.Once via AppError → buildEnvelope.
	for _, path := range locations {
		if id := tryReadValidID(path); id != "" {
			healMissingLocations(id, locations)
			return id
		}
	}

	// Slow path: nothing on disk. Take the lock, re-check (covers
	// concurrent daemon races), then derive or randomize.
	if err := os.MkdirAll(kaboomDir, 0o700); err != nil {
		return ""
	}

	var stableID string
	if err := withKaboomStateLock("install_id.lock", func() error {
		for _, path := range locations {
			if id := tryReadValidID(path); id != "" {
				stableID = id
				return nil
			}
		}

		// Prefer deterministic derivation so wiped/restored environments recover
		// the same ID instead of appearing as a new install.
		candidate, ok := deriveInstallID()
		if !ok {
			candidate = generateRandomID()
		}
		if installIDBeforePersistHook != nil {
			installIDBeforePersistHook()
		}
		// Primary write must succeed; secondary/backup writes are best-effort
		// (handled by healMissingLocations after the lock is released).
		if err := writeTokenAtomic(locations[0], candidate); err != nil {
			return err
		}
		stableID = candidate
		return nil
	}); err != nil {
		return ""
	}

	healMissingLocations(stableID, locations)
	return stableID
}

// installIDLocations returns the file paths the install_id may live at, in
// priority order: primary, primary.bak, secondary, secondary.bak. Read tries
// them in order; write fans out to all four.
func installIDLocations() []string {
	primary := filepath.Join(kaboomDir, "install_id")
	locs := []string{primary, primary + ".bak"}
	if sec := secondaryKaboomDir(); sec != "" {
		secPath := filepath.Join(sec, "install_id")
		locs = append(locs, secPath, secPath+".bak")
	}
	return locs
}

// secondaryKaboomDir returns the platform-stable mirror directory or "" if
// no usable home dir / unsupported OS / explicitly disabled. Tests override
// via secondaryDirOverride or secondaryDirDisabled.
func secondaryKaboomDir() string {
	if secondaryDirDisabled {
		return ""
	}
	if secondaryDirOverride != "" {
		return secondaryDirOverride
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Kaboom")
	case "linux":
		if x := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); x != "" {
			return filepath.Join(x, "kaboom")
		}
		return filepath.Join(home, ".local", "state", "kaboom")
	case "windows":
		if x := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); x != "" {
			return filepath.Join(x, "Kaboom")
		}
		return filepath.Join(home, "AppData", "Local", "Kaboom")
	default:
		return ""
	}
}

// tryReadValidID reads `path` and returns the trimmed content only if it
// matches `^[0-9a-f]{12}$`. Anything else (missing, IO error, garbage) maps
// to "" so the caller falls through to the next location or derivation.
func tryReadValidID(path string) string {
	id, err := readTrimmedFile(path)
	if err != nil {
		return ""
	}
	if !validInstallID(id) {
		return ""
	}
	return id
}

// validInstallID enforces the wire format: exactly 12 lowercase hex chars.
// Stricter than `len(s) > 0` so corrupted files don't propagate as IDs.
func validInstallID(s string) bool {
	if len(s) != 12 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		default:
			return false
		}
	}
	return true
}

// healMissingLocations writes id to every location whose current content
// disagrees. Best-effort: a failure on one path doesn't stop the others.
// This is the self-repair pass that makes "delete the primary file"
// recoverable on the next read.
func healMissingLocations(id string, locations []string) {
	for _, path := range locations {
		if existing, err := readTrimmedFile(path); err == nil && existing == id {
			continue
		}
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			continue
		}
		_ = writeTokenAtomic(path, id)
	}
}

// writeTokenAtomic writes id to idPath atomically: temp file in the same
// directory, Sync, Close, Rename. On any error the temp file is removed so
// we never leak partially-written state.
func writeTokenAtomic(idPath, id string) (retErr error) {
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

func withKaboomStateLock(lockName string, fn func() error) error {
	lockPath := filepath.Join(kaboomDir, lockName)
	deadline := time.Now().Add(installStateLockTimeout)
	for {
		lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			if closeErr := lockFile.Close(); closeErr != nil {
				_ = os.Remove(lockPath)
				return closeErr
			}
			defer os.Remove(lockPath)
			return fn()
		}
		if !os.IsExist(err) {
			return err
		}
		if staleLock(lockPath) {
			_ = os.Remove(lockPath)
			continue
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(installStateLockPoll)
	}
}

func staleLock(lockPath string) bool {
	info, err := os.Stat(lockPath)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) > installStateLockStale
}

func readTrimmedFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
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
	id, err := readTrimmedFile(filepath.Join(kaboomDir, "first_tool_call_install_id"))
	if err != nil {
		return ""
	}
	return id
}

func claimFirstToolCallInstallID(installID string) (bool, error) {
	if strings.TrimSpace(kaboomDir) == "" || strings.TrimSpace(installID) == "" {
		return false, nil
	}
	if err := os.MkdirAll(kaboomDir, 0o700); err != nil {
		return false, err
	}

	markerPath := filepath.Join(kaboomDir, "first_tool_call_install_id")
	claimed := false
	err := withKaboomStateLock("first_tool_call_install_id.lock", func() error {
		current, err := readTrimmedFile(markerPath)
		if err == nil && current == installID {
			return nil
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if firstToolCallBeforePersistHook != nil {
			firstToolCallBeforePersistHook()
		}
		if err := writeTokenAtomic(markerPath, installID); err != nil {
			return err
		}
		claimed = true
		return nil
	})
	return claimed, err
}

// markFirstToolCallEmittedForInstall persists first-tool state and returns true
// only the first time it is called for the current install ID.
func markFirstToolCallEmittedForInstall() bool {
	firstToolCallMu.Lock()
	defer firstToolCallMu.Unlock()

	if !cachedFirstToolCallLoaded {
		cachedFirstToolCallInstallID = loadFirstToolCallInstallID()
		cachedFirstToolCallLoaded = true
	}

	installID := GetInstallID()
	if installID == "" {
		return false
	}
	if cachedFirstToolCallInstallID == installID {
		return false
	}

	claimed, err := claimFirstToolCallInstallID(installID)
	if err != nil {
		return false
	}
	cachedFirstToolCallInstallID = installID
	return claimed
}

func generateRandomID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		// Fallback: return a zero-filled ID rather than failing.
		return "000000000000"
	}
	return hex.EncodeToString(b)
}

// Test helpers (overrideKaboomDir, overrideSecondaryDir, resetKaboomDir,
// resetInstallIDState, resetFirstToolCallState) live in helpers_test.go.
