// Purpose: Manages persistent security configuration including whitelisted origins and flagging severity.
// Why: Separates policy persistence and loading from runtime security mode management.
package security

import (
	"errors"
	"fmt"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/state"
)

type SecurityConfig struct {
	Version             string            `json:"version"`
	WhitelistedOrigins  []string          `json:"whitelisted_origins"`
	MinFlaggingSeverity string            `json:"min_flagging_severity"`
	Notes               map[string]string `json:"notes,omitempty"`
}

type SecurityAuditEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	Action       string    `json:"action"`
	Origin       string    `json:"origin,omitempty"`
	Reason       string    `json:"reason"`
	Persistent   bool      `json:"persistent"`
	Source       string    `json:"source"`
	MCPSessionID string    `json:"mcp_session_id,omitempty"`
}

var securityConfigPath = ""

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

// AddToWhitelist is intentionally manual-only.
// Security policy mutations must be reviewed and applied by a human editing the config file.
func AddToWhitelist(origin string) error {
	return blockSecurityConfigMutation("add_to_whitelist", origin, "")
}

// SetMinSeverity is intentionally manual-only.
// Security policy mutations must be reviewed and applied by a human editing the config file.
func SetMinSeverity(severity string) error {
	return blockSecurityConfigMutation("set_min_severity", "", severity)
}

// ClearWhitelist is intentionally manual-only.
// Security policy mutations must be reviewed and applied by a human editing the config file.
func ClearWhitelist() error {
	return blockSecurityConfigMutation("clear_whitelist", "", "")
}

func blockSecurityConfigMutation(action string, origin string, detail string) error {
	reason := securityConfigMutationReason()
	if detail != "" {
		reason = fmt.Sprintf("%s (%s=%q)", reason, action, detail)
	}
	err := errors.New(reason + " - " + securityConfigEditInstruction())

	LogSecurityEvent(SecurityAuditEvent{
		Timestamp:  time.Now(),
		Action:     "security_config_mutation_blocked",
		Origin:     origin,
		Reason:     err.Error(),
		Persistent: false,
		Source:     "security_config_policy",
	})

	return err
}

func securityConfigMutationReason() string {
	if IsMCPMode() {
		return "security config updates are manual-only in MCP mode and require human review"
	}
	if !IsInteractiveTerminal() {
		return "security config updates are manual-only in non-interactive environments"
	}
	return "security config updates are manual-only; interactive mutation commands are intentionally disabled"
}
