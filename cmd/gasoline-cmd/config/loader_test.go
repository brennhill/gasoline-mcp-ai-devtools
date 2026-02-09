// loader_test.go — Tests for configuration loading cascade.
// Tests priority: defaults < .gasoline.json < env vars < flags.
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	t.Parallel()
	cfg := Defaults()

	if cfg.ServerPort != 7890 {
		t.Errorf("expected default port 7890, got %d", cfg.ServerPort)
	}
	if cfg.Format != "human" {
		t.Errorf("expected default format 'human', got %q", cfg.Format)
	}
	if cfg.Timeout != 5000 {
		t.Errorf("expected default timeout 5000, got %d", cfg.Timeout)
	}
	if !cfg.AutoStartServer {
		t.Error("expected auto-start server to be true by default")
	}
	if cfg.Stream {
		t.Error("expected stream to be false by default")
	}
}

func TestLoadProjectConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Write a project config file
	configPath := filepath.Join(dir, ".gasoline.json")
	err := os.WriteFile(configPath, []byte(`{
		"server_port": 9224,
		"format": "json",
		"timeout": 30000,
		"auto_start_server": false
	}`), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg := Defaults()
	err = loadProjectConfig(&cfg, dir)
	if err != nil {
		t.Fatalf("loadProjectConfig failed: %v", err)
	}

	if cfg.ServerPort != 9224 {
		t.Errorf("expected port 9224, got %d", cfg.ServerPort)
	}
	if cfg.Format != "json" {
		t.Errorf("expected format 'json', got %q", cfg.Format)
	}
	if cfg.Timeout != 30000 {
		t.Errorf("expected timeout 30000, got %d", cfg.Timeout)
	}
	if cfg.AutoStartServer {
		t.Error("expected auto-start to be false")
	}
}

func TestLoadProjectConfigMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cfg := Defaults()
	err := loadProjectConfig(&cfg, dir)
	if err != nil {
		t.Fatalf("missing config should not error, got: %v", err)
	}

	// Should keep defaults
	if cfg.ServerPort != 7890 {
		t.Errorf("expected default port, got %d", cfg.ServerPort)
	}
}

func TestLoadProjectConfigInvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	configPath := filepath.Join(dir, ".gasoline.json")
	err := os.WriteFile(configPath, []byte(`{bad json`), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg := Defaults()
	err = loadProjectConfig(&cfg, dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadEnvVars(t *testing.T) {
	// Cannot be parallel due to env manipulation
	t.Setenv("GASOLINE_PORT", "9225")
	t.Setenv("GASOLINE_FORMAT", "csv")
	t.Setenv("GASOLINE_TIMEOUT", "60000")
	t.Setenv("GASOLINE_NO_AUTO_START", "1")

	cfg := Defaults()
	loadEnvVars(&cfg)

	if cfg.ServerPort != 9225 {
		t.Errorf("expected port 9225, got %d", cfg.ServerPort)
	}
	if cfg.Format != "csv" {
		t.Errorf("expected format 'csv', got %q", cfg.Format)
	}
	if cfg.Timeout != 60000 {
		t.Errorf("expected timeout 60000, got %d", cfg.Timeout)
	}
	if cfg.AutoStartServer {
		t.Error("expected auto-start to be false")
	}
}

func TestLoadEnvVarsInvalidPort(t *testing.T) {
	t.Setenv("GASOLINE_PORT", "notanumber")

	cfg := Defaults()
	loadEnvVars(&cfg)

	// Should keep default on invalid input
	if cfg.ServerPort != 7890 {
		t.Errorf("expected default port on invalid env, got %d", cfg.ServerPort)
	}
}

func TestConfigPriorityOrder(t *testing.T) {
	// Cannot be parallel due to env manipulation
	dir := t.TempDir()

	// Project config sets port=9224, format=json
	configPath := filepath.Join(dir, ".gasoline.json")
	err := os.WriteFile(configPath, []byte(`{
		"server_port": 9224,
		"format": "json"
	}`), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Env var overrides port to 9225
	t.Setenv("GASOLINE_PORT", "9225")

	cfg, err := Load(dir, nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Env should override project config for port
	if cfg.ServerPort != 9225 {
		t.Errorf("expected env port 9225 to override project, got %d", cfg.ServerPort)
	}
	// Project config should apply for format (no env override)
	if cfg.Format != "json" {
		t.Errorf("expected project format 'json', got %q", cfg.Format)
	}
}

func TestFlagOverrides(t *testing.T) {
	dir := t.TempDir()

	// Project config sets format=json
	configPath := filepath.Join(dir, ".gasoline.json")
	err := os.WriteFile(configPath, []byte(`{"format": "json"}`), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Flags override everything
	overrides := &FlagOverrides{
		ServerPort: intPtr(9999),
		Format:     strPtr("csv"),
		Timeout:    intPtr(1000),
	}

	cfg, err := Load(dir, overrides)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.ServerPort != 9999 {
		t.Errorf("expected flag port 9999, got %d", cfg.ServerPort)
	}
	if cfg.Format != "csv" {
		t.Errorf("expected flag format 'csv', got %q", cfg.Format)
	}
	if cfg.Timeout != 1000 {
		t.Errorf("expected flag timeout 1000, got %d", cfg.Timeout)
	}
}

func TestValidFormats(t *testing.T) {
	t.Parallel()

	valid := []string{"human", "json", "csv"}
	for _, f := range valid {
		cfg := Config{ServerPort: 7890, Format: f}
		if err := cfg.Validate(); err != nil {
			t.Errorf("format %q should be valid, got: %v", f, err)
		}
	}

	cfg := Config{ServerPort: 7890, Format: "xml"}
	if err := cfg.Validate(); err == nil {
		t.Error("format 'xml' should be invalid")
	}
}

func TestValidatePortRange(t *testing.T) {
	t.Parallel()

	cfg := Config{ServerPort: 0, Format: "human"}
	if err := cfg.Validate(); err == nil {
		t.Error("port 0 should be invalid")
	}

	cfg = Config{ServerPort: 70000, Format: "human"}
	if err := cfg.Validate(); err == nil {
		t.Error("port 70000 should be invalid")
	}

	cfg = Config{ServerPort: 7890, Format: "human"}
	if err := cfg.Validate(); err != nil {
		t.Errorf("port 7890 should be valid, got: %v", err)
	}
}

func TestLoadGlobalConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Write a global config file
	err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{
		"server_port": 9226,
		"format": "csv"
	}`), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg := Defaults()
	err = loadGlobalConfig(&cfg, dir)
	if err != nil {
		t.Fatalf("loadGlobalConfig failed: %v", err)
	}

	if cfg.ServerPort != 9226 {
		t.Errorf("expected port 9226, got %d", cfg.ServerPort)
	}
	if cfg.Format != "csv" {
		t.Errorf("expected format 'csv', got %q", cfg.Format)
	}
}

// Helper functions for creating pointers to values
func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }
