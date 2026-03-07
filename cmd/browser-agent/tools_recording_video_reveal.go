// Purpose: Handles path validation and OS reveal actions for /recordings/reveal.
// Why: Keeps side-effectful file manager behavior isolated from upload/list logic.
// Docs: docs/features/feature/tab-recording/index.md

package main

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// resolveRevealPath resolves and validates a path against recordings directories.
// Returns the resolved path, an HTTP status code, and an error message.
// Status 0 means success.
func resolveRevealPath(rawPath string, dirs []string) (string, int, string) {
	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		return "", http.StatusBadRequest, "Invalid path"
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absPath); resolveErr == nil {
		absPath = resolved
	}

	if !isPathInAnyDir(absPath, dirs) {
		return "", http.StatusForbidden, "Path not within recordings directory"
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", http.StatusNotFound, "File not found"
	}

	return absPath, 0, ""
}

// isPathInAnyDir returns true if absPath is within any of the given directories.
func isPathInAnyDir(absPath string, dirs []string) bool {
	for _, dir := range dirs {
		if pathWithinDir(absPath, dir) {
			return true
		}
	}
	return false
}

// validateRevealPath checks that the path is valid and within a recordings directory.
// Returns the resolved absolute path or writes an HTTP error and returns empty string.
func validateRevealPath(w http.ResponseWriter, rawPath string) string {
	dirs := recordingsReadDirs()
	if len(dirs) == 0 {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Could not resolve recordings directory"})
		return ""
	}

	absPath, status, errMsg := resolveRevealPath(rawPath, dirs)
	if status != 0 {
		jsonResponse(w, status, map[string]string{"error": errMsg})
		return ""
	}

	return absPath
}

type revealCommandRunner func(name string, args ...string) error

func defaultRevealCommandRunner(name string, args ...string) error {
	return exec.Command(name, args...).Run() // #nosec G204 -- name is from revealCommandForOS (hardcoded "open"/"explorer"/"xdg-open")
}

func revealCommandForOS(goos, absPath string) (string, []string) {
	switch goos {
	case "darwin":
		return "open", []string{"-R", absPath}
	case "windows":
		return "explorer", []string{"/select,", absPath}
	default:
		return "xdg-open", []string{filepath.Dir(absPath)}
	}
}

// revealInFileManagerWithRunner separates command selection from execution so
// tests can verify behavior without opening Finder/Explorer on the developer machine.
func revealInFileManagerWithRunner(goos, absPath string, runner revealCommandRunner) error {
	name, args := revealCommandForOS(goos, absPath)
	return runner(name, args...)
}

// revealInFileManager opens the platform file manager highlighting the given path.
func revealInFileManager(absPath string) error {
	return revealInFileManagerWithRunner(runtime.GOOS, absPath, defaultRevealCommandRunner)
}

// handleRevealRecording handles POST /recordings/reveal — opens Finder/Explorer to the file.
func handleRevealRecording(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if body.Path == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing path"})
		return
	}

	absPath := validateRevealPath(w, body.Path)
	if absPath == "" {
		return
	}

	if err := revealInFileManager(absPath); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to reveal file: " + err.Error()})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "revealed", "path": absPath})
}
