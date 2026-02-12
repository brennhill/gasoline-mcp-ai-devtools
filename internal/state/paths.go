// Package state centralizes filesystem locations for Gasoline runtime artifacts.
package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// StateDirEnv overrides the default runtime state root.
	StateDirEnv = "GASOLINE_STATE_DIR"

	xdgStateHomeEnv = "XDG_STATE_HOME"
	appName         = "gasoline"
)

// RootDir returns the runtime state root for Gasoline.
// Resolution order:
//  1. GASOLINE_STATE_DIR (if set)
//  2. XDG_STATE_HOME/gasoline (if XDG_STATE_HOME is set)
//  3. os.UserConfigDir()/gasoline (cross-platform fallback)
func RootDir() (string, error) {
	if override := strings.TrimSpace(os.Getenv(StateDirEnv)); override != "" {
		return normalizePath(override)
	}

	if xdg := strings.TrimSpace(os.Getenv(xdgStateHomeEnv)); xdg != "" {
		root, err := normalizePath(xdg)
		if err != nil {
			return "", err
		}
		return filepath.Join(root, appName), nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine user config directory: %w", err)
	}
	root, err := normalizePath(configDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, appName), nil
}

// LegacyRootDir returns the historical runtime root used by earlier versions.
func LegacyRootDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(homeDir, ".gasoline"), nil
}

// LogsDir returns the logs directory under RootDir.
func LogsDir() (string, error) {
	return InRoot("logs")
}

// DefaultLogFile returns the default structured log file path.
func DefaultLogFile() (string, error) {
	return InRoot("logs", "gasoline.jsonl")
}

// LegacyDefaultLogFile returns the previous default log file path.
func LegacyDefaultLogFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(homeDir, "gasoline-logs.jsonl"), nil
}

// CrashLogFile returns the panic crash log file path.
func CrashLogFile() (string, error) {
	return InRoot("logs", "crash.log")
}

// LegacyCrashLogFile returns the previous crash log file path.
func LegacyCrashLogFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(homeDir, "gasoline-crash.log"), nil
}

// PIDFile returns the PID file path for the given server port.
func PIDFile(port int) (string, error) {
	return InRoot("run", "gasoline-"+strconv.Itoa(port)+".pid")
}

// LegacyPIDFile returns the historical PID file path for the given server port.
func LegacyPIDFile(port int) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(homeDir, ".gasoline-"+strconv.Itoa(port)+".pid"), nil
}

// RecordingsDir returns the recordings directory.
func RecordingsDir() (string, error) {
	return InRoot("recordings")
}

// ScreenshotsDir returns the screenshots directory.
func ScreenshotsDir() (string, error) {
	return InRoot("screenshots")
}

// LegacyRecordingsDir returns the historical recordings directory.
func LegacyRecordingsDir() (string, error) {
	root, err := LegacyRootDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "recordings"), nil
}

// SettingsFile returns the extension settings cache file path.
func SettingsFile() (string, error) {
	return InRoot("settings", "extension-settings.json")
}

// LegacySettingsFile returns the historical settings cache file path.
func LegacySettingsFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(homeDir, ".gasoline-settings.json"), nil
}

// SecurityConfigFile returns the security configuration path.
func SecurityConfigFile() (string, error) {
	return InRoot("security", "security.json")
}

// LegacySecurityConfigFile returns the historical security config path.
func LegacySecurityConfigFile() (string, error) {
	root, err := LegacyRootDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "security.json"), nil
}

// InRoot returns a path rooted under RootDir with additional path elements.
func InRoot(parts ...string) (string, error) {
	root, err := RootDir()
	if err != nil {
		return "", err
	}
	all := make([]string, 0, len(parts)+1)
	all = append(all, root)
	all = append(all, parts...)
	return filepath.Join(all...), nil
}

func normalizePath(path string) (string, error) {
	if path == "" {
		return "", errors.New("empty path")
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("cannot resolve path %q: %w", path, err)
	}
	return filepath.Clean(absPath), nil
}
