// loader.go — Configuration loading with priority cascade.
// Priority: defaults < global config < project config < env vars < flags.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds all resolved configuration values.
type Config struct {
	ServerPort      int    `json:"server_port"`
	Format          string `json:"format"`
	Timeout         int    `json:"timeout"`
	AutoStartServer bool   `json:"auto_start_server"`
	Stream          bool   `json:"stream"`
}

// FlagOverrides holds values explicitly set via command-line flags.
// Nil pointer means the flag was not set (so lower-priority values are kept).
type FlagOverrides struct {
	ServerPort      *int
	Format          *string
	Timeout         *int
	AutoStartServer *bool
	Stream          *bool
}

// Defaults returns the base configuration with sensible defaults.
func Defaults() Config {
	return Config{
		ServerPort:      7890,
		Format:          "human",
		Timeout:         5000,
		AutoStartServer: true,
		Stream:          false,
	}
}

// Load builds the final configuration by applying the priority cascade:
// defaults < global (~/.gasoline/config.json) < project (.gasoline.json) < env vars < flags.
func Load(projectDir string, flags *FlagOverrides) (Config, error) {
	cfg := Defaults()

	// Load global config from ~/.gasoline/config.json
	home, err := os.UserHomeDir()
	if err == nil {
		gasolineDir := filepath.Join(home, ".gasoline")
		_ = loadGlobalConfig(&cfg, gasolineDir)
	}

	// Load project config from .gasoline.json in the given directory
	if err := loadProjectConfig(&cfg, projectDir); err != nil {
		return cfg, fmt.Errorf("project config: %w", err)
	}

	// Apply environment variables
	loadEnvVars(&cfg)

	// Apply flag overrides (highest priority)
	if flags != nil {
		applyFlags(&cfg, flags)
	}

	// Validate final config
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

// loadGlobalConfig reads ~/.gasoline/config.json if it exists.
func loadGlobalConfig(cfg *Config, gasolineDir string) error {
	return loadJSONFile(cfg, filepath.Join(gasolineDir, "config.json"))
}

// loadProjectConfig reads .gasoline.json from the given directory if it exists.
func loadProjectConfig(cfg *Config, dir string) error {
	return loadJSONFile(cfg, filepath.Join(dir, ".gasoline.json"))
}

// loadJSONFile reads a JSON config file and merges non-zero values into cfg.
func loadJSONFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Missing config file is fine
		}
		return err
	}

	var fileCfg fileConfig
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	// Only override fields that are explicitly set in the file
	if fileCfg.ServerPort != nil {
		cfg.ServerPort = *fileCfg.ServerPort
	}
	if fileCfg.Format != nil {
		cfg.Format = *fileCfg.Format
	}
	if fileCfg.Timeout != nil {
		cfg.Timeout = *fileCfg.Timeout
	}
	if fileCfg.AutoStartServer != nil {
		cfg.AutoStartServer = *fileCfg.AutoStartServer
	}

	return nil
}

// fileConfig uses pointers to distinguish "not set" from zero values.
type fileConfig struct {
	ServerPort      *int    `json:"server_port"`
	Format          *string `json:"format"`
	Timeout         *int    `json:"timeout"`
	AutoStartServer *bool   `json:"auto_start_server"`
}

// loadEnvVars applies environment variable overrides.
func loadEnvVars(cfg *Config) {
	if v := os.Getenv("GASOLINE_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.ServerPort = port
		}
	}
	if v := os.Getenv("GASOLINE_FORMAT"); v != "" {
		cfg.Format = v
	}
	if v := os.Getenv("GASOLINE_TIMEOUT"); v != "" {
		if timeout, err := strconv.Atoi(v); err == nil {
			cfg.Timeout = timeout
		}
	}
	if os.Getenv("GASOLINE_NO_AUTO_START") == "1" {
		cfg.AutoStartServer = false
	}
}

// applyFlags applies command-line flag overrides (highest priority).
func applyFlags(cfg *Config, flags *FlagOverrides) {
	if flags.ServerPort != nil {
		cfg.ServerPort = *flags.ServerPort
	}
	if flags.Format != nil {
		cfg.Format = *flags.Format
	}
	if flags.Timeout != nil {
		cfg.Timeout = *flags.Timeout
	}
	if flags.AutoStartServer != nil {
		cfg.AutoStartServer = *flags.AutoStartServer
	}
	if flags.Stream != nil {
		cfg.Stream = *flags.Stream
	}
}

// Validate checks that configuration values are within acceptable ranges.
func (c Config) Validate() error {
	if c.ServerPort < 1 || c.ServerPort > 65535 {
		return fmt.Errorf("server_port must be 1-65535, got %d", c.ServerPort)
	}

	validFormats := map[string]bool{"human": true, "json": true, "csv": true}
	if !validFormats[c.Format] {
		return fmt.Errorf("format must be human, json, or csv, got %q", c.Format)
	}

	return nil
}
