// exit_diagnostics.go — Best-effort crash/shutdown diagnostics persisted to disk.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/dev-console/dev-console/internal/state"
)

// appendExitDiagnostic writes a structured exit diagnostic entry to the first
// writable crash-log candidate and returns the path used, or "" on total failure.
func appendExitDiagnostic(event string, extra map[string]any) string {
	entry := map[string]any{
		"type":       "lifecycle",
		"event":      event,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"pid":        os.Getpid(),
		"version":    version,
		"go_version": runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
	}
	for k, v := range extra {
		entry[k] = v
	}

	path, err := writeDiagnosticToCandidates(crashLogCandidates(), entry)
	if err != nil {
		return ""
	}
	return path
}

func crashLogCandidates() []string {
	seen := map[string]struct{}{}
	candidates := make([]string, 0, 3)
	add := func(path string) {
		if path == "" {
			return
		}
		if _, exists := seen[path]; exists {
			return
		}
		seen[path] = struct{}{}
		candidates = append(candidates, path)
	}

	if p, err := state.CrashLogFile(); err == nil {
		add(p)
	}
	if p, err := state.LegacyCrashLogFile(); err == nil {
		add(p)
	}
	add(filepath.Join(os.TempDir(), "gasoline-crash.log"))
	return candidates
}

func writeDiagnosticToCandidates(candidates []string, entry map[string]any) (string, error) {
	if len(candidates) == 0 {
		return "", fmt.Errorf("no crash-log candidates")
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return "", err
	}

	var lastErr error
	for _, path := range candidates {
		if path == "" {
			continue
		}
		// #nosec G301 -- diagnostics dir needs group read/execute for support workflows
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			lastErr = err
			continue
		}
		// #nosec G304 -- crash path is derived from local runtime state paths only
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // nosemgrep: go_filesystem_rule-fileread
		if err != nil {
			lastErr = err
			continue
		}
		if _, err := f.Write(data); err != nil {
			lastErr = err
			_ = f.Close()
			continue
		}
		if _, err := f.Write([]byte{'\n'}); err != nil {
			lastErr = err
			_ = f.Close()
			continue
		}
		if err := f.Close(); err != nil {
			lastErr = err
			continue
		}
		return path, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no writable crash-log candidates")
	}
	return "", lastErr
}
