// Security boundary: LLM trust model for config persistence
// Implements security boundary between untrusted LLM tool calls and trusted persistent configuration.
// See: docs/specs/security-boundary-llm-trust.md
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ============================================
// Security Boundary: LLM Trust Model
// ============================================
// This file implements the security boundary between LLM tool calls
// (untrusted) and persistent security configuration (trusted).
// See: docs/specs/security-boundary-llm-trust.md

// ============================================
// Mode Detection
// ============================================

var (
	isMCPMode     bool
	isInteractive bool
	modeMu        sync.RWMutex
)

// InitMode detects whether we're running as MCP server or interactive CLI
func InitMode() {
	modeMu.Lock()
	defer modeMu.Unlock()

	// Detect if running as MCP server (stdin/stdout used for JSON-RPC)
	if os.Getenv("MCP_MODE") == "1" {
		isMCPMode = true
		isInteractive = false
		return
	}

	// TODO: Add isStdinStdoutPipe() detection
	isMCPMode = false
	isInteractive = true // For now, assume interactive if not MCP mode
}

// IsMCPMode returns true if running as MCP server
func IsMCPMode() bool {
	modeMu.RLock()
	defer modeMu.RUnlock()
	return isMCPMode
}

// IsInteractiveTerminal returns true if running in interactive CLI mode
func IsInteractiveTerminal() bool {
	modeMu.RLock()
	defer modeMu.RUnlock()
	return isInteractive
}

// ============================================
// Security Config Types
// ============================================

// SecurityConfig represents persistent security configuration
type SecurityConfig struct {
	Version             string            `json:"version"`
	WhitelistedOrigins  []string          `json:"whitelisted_origins"`
	MinFlaggingSeverity string            `json:"min_flagging_severity"`
	Notes               map[string]string `json:"notes,omitempty"`
}

// SecurityAuditEvent represents a security decision for audit logging
type SecurityAuditEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	Action       string    `json:"action"`           // "csp_generated", "flag_suppressed", "whitelist_override"
	Origin       string    `json:"origin,omitempty"` // Which origin was affected
	Reason       string    `json:"reason"`           // Why this action was taken
	Persistent   bool      `json:"persistent"`       // false for session-only overrides
	Source       string    `json:"source"`           // "mcp", "cli", "config_file"
	MCPSessionID string    `json:"mcp_session_id,omitempty"`
}

var (
	securityConfigPath = ""
	securityAuditLog   []SecurityAuditEvent
	securityAuditMu    sync.Mutex
)

// ============================================
// Security Config Path Management
// ============================================

func getSecurityConfigPath() string {
	if securityConfigPath != "" {
		return securityConfigPath
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(homeDir, ".gasoline", "security.json")
}

func setSecurityConfigPath(path string) {
	securityConfigPath = path
}

// ============================================
// Security Config Modification Guards
// ============================================

// AddToWhitelist adds an origin to the persistent whitelist
// BLOCKED in MCP mode - requires human review
func AddToWhitelist(origin string) error {
	if IsMCPMode() {
		return errors.New("security config updates require human review - edit ~/.gasoline/security.json manually")
	}

	if !IsInteractiveTerminal() {
		return errors.New("not in interactive mode - edit ~/.gasoline/security.json manually")
	}

	// TODO: Implement interactive confirmation and config file update
	return fmt.Errorf("AddToWhitelist not yet fully implemented for origin: %s", origin)
}

// SetMinSeverity sets the minimum severity threshold for security flagging
// BLOCKED in MCP mode - requires human review
func SetMinSeverity(severity string) error {
	if IsMCPMode() {
		return errors.New("security config updates require human review - edit ~/.gasoline/security.json manually")
	}

	if !IsInteractiveTerminal() {
		return errors.New("not in interactive mode - edit ~/.gasoline/security.json manually")
	}

	// TODO: Implement interactive confirmation and config file update
	return fmt.Errorf("SetMinSeverity not yet fully implemented for severity: %s", severity)
}

// ClearWhitelist clears all whitelist entries
// BLOCKED in MCP mode - requires human review
func ClearWhitelist() error {
	if IsMCPMode() {
		return errors.New("security config updates require human review - edit ~/.gasoline/security.json manually")
	}

	if !IsInteractiveTerminal() {
		return errors.New("not in interactive mode - edit ~/.gasoline/security.json manually")
	}

	// TODO: Implement interactive confirmation and config file update
	return errors.New("ClearWhitelist not yet fully implemented")
}

// ============================================
// Audit Logging
// ============================================

// LogSecurityEvent appends a security audit event
func LogSecurityEvent(event SecurityAuditEvent) {
	securityAuditMu.Lock()
	defer securityAuditMu.Unlock()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	securityAuditLog = append(securityAuditLog, event)

	// TODO: Persist to ~/.gasoline/security-audit.jsonl
}

// GetSecurityAuditEvents returns all security audit events
func GetSecurityAuditEvents() []SecurityAuditEvent {
	securityAuditMu.Lock()
	defer securityAuditMu.Unlock()

	// Return a copy to prevent external modification
	events := make([]SecurityAuditEvent, len(securityAuditLog))
	copy(events, securityAuditLog)
	return events
}

// ClearSecurityAuditEvents clears all audit events (for testing)
func ClearSecurityAuditEvents() {
	securityAuditMu.Lock()
	defer securityAuditMu.Unlock()
	securityAuditLog = nil
}

// ============================================
// CSP Security Boundary Types
// ============================================
// Note: CSPGenerator and CSPParams are defined in csp.go
// These are additional types for security boundary enforcement

// CSPAudit contains audit information for CSP generation
type CSPAudit struct {
	SessionOverrides    []string `json:"session_overrides,omitempty"`
	PersistentWhitelist []string `json:"persistent_whitelist,omitempty"`
	OverrideSource      string   `json:"override_source,omitempty"`
}
