package main

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestExtractGoBuildID(t *testing.T) {
	t.Parallel()

	data := []byte("abc" + goBuildIDPrefix + "build-id-123\"xyz")
	if got := extractGoBuildID(data); got != "build-id-123" {
		t.Fatalf("extractGoBuildID() = %q, want %q", got, "build-id-123")
	}

	if got := extractGoBuildID([]byte("no build id here")); got != "" {
		t.Fatalf("extractGoBuildID() = %q, want empty", got)
	}

	if got := extractGoBuildID([]byte("abc" + goBuildIDPrefix + "missing-quote")); got != "" {
		t.Fatalf("extractGoBuildID() missing quote = %q, want empty", got)
	}
}

func TestBridgeLaunchFingerprint(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	exePath := tmp + "/gasoline-mcp-test"
	content := []byte("header" + goBuildIDPrefix + "test-build-id\"tail")
	if err := os.WriteFile(exePath, content, 0o755); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	oldGetter := getBridgeExecutablePath
	t.Cleanup(func() {
		getBridgeExecutablePath = oldGetter
	})
	getBridgeExecutablePath = func() (string, error) { return exePath, nil }

	fingerprint := bridgeLaunchFingerprint()
	if got := fingerprint["binary_path"]; got != exePath {
		t.Fatalf("binary_path = %v, want %s", got, exePath)
	}
	if got := fingerprint["binary_version"]; got != version {
		t.Fatalf("binary_version = %v, want %s", got, version)
	}
	if got := fingerprint["binary_build_id"]; got != "test-build-id" {
		t.Fatalf("binary_build_id = %v, want test-build-id", got)
	}

	sha, ok := fingerprint["binary_sha256"].(string)
	if !ok || len(sha) != 64 {
		t.Fatalf("binary_sha256 = %v, want 64-char hex string", fingerprint["binary_sha256"])
	}
	if _, exists := fingerprint["binary_path_error"]; exists {
		t.Fatalf("unexpected binary_path_error: %v", fingerprint["binary_path_error"])
	}
	if _, exists := fingerprint["binary_build_id_error"]; exists {
		t.Fatalf("unexpected binary_build_id_error: %v", fingerprint["binary_build_id_error"])
	}
	if _, exists := fingerprint["binary_sha256_error"]; exists {
		t.Fatalf("unexpected binary_sha256_error: %v", fingerprint["binary_sha256_error"])
	}
}

func TestBridgeLaunchFingerprint_PathError(t *testing.T) {
	t.Parallel()

	oldGetter := getBridgeExecutablePath
	t.Cleanup(func() {
		getBridgeExecutablePath = oldGetter
	})
	getBridgeExecutablePath = func() (string, error) { return "", errors.New("boom") }

	fingerprint := bridgeLaunchFingerprint()
	if got := fingerprint["binary_path"]; got != "" {
		t.Fatalf("binary_path = %v, want empty", got)
	}
	if got, ok := fingerprint["binary_build_id"].(string); !ok || got != "unknown" {
		t.Fatalf("binary_build_id = %v, want unknown", fingerprint["binary_build_id"])
	}
	if got, ok := fingerprint["binary_sha256"].(string); !ok || got != "unknown" {
		t.Fatalf("binary_sha256 = %v, want unknown", fingerprint["binary_sha256"])
	}
	pathErr, _ := fingerprint["binary_path_error"].(string)
	if !strings.Contains(pathErr, "boom") {
		t.Fatalf("binary_path_error = %q, want contains boom", pathErr)
	}
}
