// Purpose: Handles security scanner configuration loading, defaults, and policy persistence boundaries.
// Why: Ensures scanners run with explicit, auditable config rather than scattered implicit constants.
// Docs: docs/features/feature/security-hardening/index.md

package security

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/state"
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

// InitMode sets runtime trust-mode flags used to gate config mutation APIs.
//
// Invariants:
// - modeMu protects isMCPMode/isInteractive as a paired state.
// - MCP mode and interactive mode are mutually exclusive in current model.
func InitMode() {
	modeMu.Lock()
	defer modeMu.Unlock()

	// Detect if running as MCP server (stdin/stdout used for JSON-RPC)
	if os.Getenv("MCP_MODE") == "1" {
		isMCPMode = true
		isInteractive = false
		return
	}

	// TODO(future): Add isStdinStdoutPipe() detection for better MCP mode detection
	// Currently, we assume interactive mode when not explicitly in MCP mode via env var
	isMCPMode = false
	isInteractive = true
}

// IsMCPMode returns whether runtime is operating in MCP server mode.
func IsMCPMode() bool {
	modeMu.RLock()
	defer modeMu.RUnlock()
	return isMCPMode
}

// IsInteractiveTerminal returns whether runtime is operating in local interactive mode.
func IsInteractiveTerminal() bool {
	modeMu.RLock()
	defer modeMu.RUnlock()
	return isInteractive
}

// ============================================
// Security Config Types
// ============================================

// SecurityConfig models persisted human-approved security policy knobs.
//
// Invariants:
// - Values are trusted only when loaded from operator-controlled filesystem config.
type SecurityConfig struct {
	Version             string            `json:"version"`
	WhitelistedOrigins  []string          `json:"whitelisted_origins"`
	MinFlaggingSeverity string            `json:"min_flagging_severity"`
	Notes               map[string]string `json:"notes,omitempty"`
}

// SecurityAuditEvent captures one policy-impacting security decision.
//
// Invariants:
// - Events are append-only and timestamped at insertion when missing.
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

// getSecurityConfigPath resolves primary config path with legacy fallback.
//
// Failure semantics:
// - Returns empty string when no path can be resolved; callers must emit manual guidance.
func getSecurityConfigPath() string {
	if securityConfigPath != "" {
		return securityConfigPath
	}

	path, err := state.SecurityConfigFile()
	if err != nil {
		if legacyPath, legacyErr := state.LegacySecurityConfigFile(); legacyErr == nil {
			return legacyPath
		}
		return ""
	}

	return path
}

// setSecurityConfigPath overrides config location (primarily for tests).
func setSecurityConfigPath(path string) {
	securityConfigPath = path
}

func securityConfigEditInstruction() string {
	path := getSecurityConfigPath()
	if path == "" {
		return "Edit the security configuration file manually"
	}
	return "Edit " + path + " manually"
}

// ============================================
// Security Config Modification Guards
// ============================================

// AddToWhitelist is a guarded mutation entrypoint for persistent origin allowlisting.
//
// Failure semantics:
// - Always rejected in MCP mode or non-interactive contexts to preserve trust boundary.
// - Currently returns not-implemented error even in interactive mode.
func AddToWhitelist(origin string) error {
	if IsMCPMode() {
		return errors.New("security config updates require human review - " + securityConfigEditInstruction())
	}

	if !IsInteractiveTerminal() {
		return errors.New("not in interactive mode - " + securityConfigEditInstruction())
	}

	// TODO(future): Implement interactive confirmation and config file update
	// For now, users must manually edit the config file.
	return fmt.Errorf("AddToWhitelist not yet fully implemented for origin: %s", origin)
}

// SetMinSeverity is a guarded mutation entrypoint for persistent severity policy.
//
// Failure semantics:
// - Always rejected in MCP mode or non-interactive contexts.
// - Currently returns not-implemented error even in interactive mode.
func SetMinSeverity(severity string) error {
	if IsMCPMode() {
		return errors.New("security config updates require human review - " + securityConfigEditInstruction())
	}

	if !IsInteractiveTerminal() {
		return errors.New("not in interactive mode - " + securityConfigEditInstruction())
	}

	// TODO(future): Implement interactive confirmation and config file update
	// For now, users must manually edit the config file.
	return fmt.Errorf("SetMinSeverity not yet fully implemented for severity: %s", severity)
}

// ClearWhitelist is a guarded destructive mutation for persistent allowlist state.
//
// Failure semantics:
// - Always rejected in MCP mode or non-interactive contexts.
// - Currently returns not-implemented error in interactive mode.
func ClearWhitelist() error {
	if IsMCPMode() {
		return errors.New("security config updates require human review - " + securityConfigEditInstruction())
	}

	if !IsInteractiveTerminal() {
		return errors.New("not in interactive mode - " + securityConfigEditInstruction())
	}

	// TODO(future): Implement interactive confirmation and config file update
	// For now, users must manually edit the config file.
	return errors.New("ClearWhitelist not yet fully implemented")
}

// ============================================
// Audit Logging
// ============================================

// LogSecurityEvent appends one in-memory audit event.
//
// Invariants:
// - securityAuditLog is mutated only under securityAuditMu.
//
// Failure semantics:
// - If timestamp missing, current time is assigned to preserve ordering semantics.
func LogSecurityEvent(event SecurityAuditEvent) {
	securityAuditMu.Lock()
	defer securityAuditMu.Unlock()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	securityAuditLog = append(securityAuditLog, event)

	// TODO(future): Persist to runtime state security-audit.jsonl for audit trail across sessions
	// Currently, audit log is in-memory only and cleared on restart
}

// GetSecurityAuditEvents returns a detached copy of audit history.
//
// Invariants:
// - Caller cannot mutate internal audit log state via returned slice.
func GetSecurityAuditEvents() []SecurityAuditEvent {
	securityAuditMu.Lock()
	defer securityAuditMu.Unlock()

	// Return a copy to prevent external modification
	events := make([]SecurityAuditEvent, len(securityAuditLog))
	copy(events, securityAuditLog)
	return events
}

// ClearSecurityAuditEvents resets in-memory audit history.
//
// Failure semantics:
// - Safe to call when log is already empty.
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
