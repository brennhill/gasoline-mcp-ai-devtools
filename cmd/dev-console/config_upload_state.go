// Purpose: Validates upload security settings and resolves runtime state/log paths.
// Why: Keeps filesystem/security setup concerns separate from CLI parsing and early mode dispatch.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/state"
)

// initUploadSecurity validates upload security configuration from CLI flags.
// Always defaults to ~/gasoline-upload-dir when no --upload-dir is specified.
// When --enable-os-upload-automation is NOT set and the dir can't be created, falls back gracefully.
func initUploadSecurity(enabled bool, dir string, denyPatterns multiFlag) {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			if enabled {
				fmt.Fprintf(os.Stderr, "[gasoline] Cannot determine home directory for default upload dir: %v\n", err)
				os.Exit(1)
			}
			uploadSecurityConfig = &UploadSecurity{}
			return
		}
		dir = filepath.Join(home, "gasoline-upload-dir")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			if enabled {
				fmt.Fprintf(os.Stderr, "[gasoline] Cannot create default upload dir %s: %v\n", dir, err)
				os.Exit(1)
			}
			uploadSecurityConfig = &UploadSecurity{}
			return
		}
	}
	sec, err := ValidateUploadDir(dir, denyPatterns)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Upload security validation failed: %v\n", err)
		os.Exit(1)
	}
	uploadSecurityConfig = sec
}

// validatePort ensures the port is within the valid TCP range.
func validatePort(port int) {
	if port < 1 || port > 65535 {
		fmt.Fprintf(os.Stderr, "[gasoline] Invalid port: %d (must be 1-65535)\n", port)
		os.Exit(1)
	}
}

// normalizeStateDir resolves the --state-dir flag to an absolute path and exports it.
func normalizeStateDir(stateDir *string) {
	if *stateDir == "" {
		return
	}
	absStateDir, err := filepath.Abs(*stateDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Invalid --state-dir: %v\n", err)
		os.Exit(1)
	}
	*stateDir = filepath.Clean(absStateDir)
	if err := os.Setenv(state.StateDirEnv, *stateDir); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Failed to set %s: %v\n", state.StateDirEnv, err)
		os.Exit(1)
	}
}

// applyParallelModeStateDir auto-isolates runtime state when --parallel is enabled
// and no explicit --state-dir is provided.
func applyParallelModeStateDir(parallel bool, stateDir *string) error {
	if !parallel {
		return nil
	}
	if strings.TrimSpace(*stateDir) != "" {
		return nil
	}

	root, err := state.RootDir()
	if err != nil {
		return fmt.Errorf("cannot resolve runtime state root: %w", err)
	}
	generated := filepath.Join(root, "parallel", fmt.Sprintf("run-%d-%d", time.Now().UnixNano(), os.Getpid()))
	if err := os.MkdirAll(generated, 0o750); err != nil {
		return fmt.Errorf("cannot create parallel state dir %q: %w", generated, err)
	}
	*stateDir = filepath.Clean(generated)
	if err := os.Setenv(state.StateDirEnv, *stateDir); err != nil {
		return fmt.Errorf("failed to set %s: %w", state.StateDirEnv, err)
	}
	startupWarnings = append(startupWarnings, fmt.Sprintf("parallel_mode_state_dir_auto: %s", *stateDir))
	return nil
}

// resolveDefaultLogFile sets the log file to the runtime state directory default if empty.
func resolveDefaultLogFile(logFile *string) {
	if *logFile != "" {
		return
	}
	defaultLogFile, err := state.DefaultLogFile()
	if err != nil {
		fallback := filepath.Join(os.TempDir(), "gasoline", "logs", "gasoline.jsonl")
		startupWarnings = append(startupWarnings, fmt.Sprintf("state_dir_unwritable: %v; falling back to %s", err, fallback))
		*logFile = fallback
		return
	}
	*logFile = defaultLogFile
}
