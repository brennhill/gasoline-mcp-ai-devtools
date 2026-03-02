package security

import (
	"errors"
	"fmt"
	"time"

	"github.com/dev-console/dev-console/internal/state"
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

func AddToWhitelist(origin string) error {
	if IsMCPMode() {
		return errors.New("security config updates require human review - " + securityConfigEditInstruction())
	}
	if !IsInteractiveTerminal() {
		return errors.New("not in interactive mode - " + securityConfigEditInstruction())
	}
	return fmt.Errorf("AddToWhitelist not yet fully implemented for origin: %s", origin)
}

func SetMinSeverity(severity string) error {
	if IsMCPMode() {
		return errors.New("security config updates require human review - " + securityConfigEditInstruction())
	}
	if !IsInteractiveTerminal() {
		return errors.New("not in interactive mode - " + securityConfigEditInstruction())
	}
	return fmt.Errorf("SetMinSeverity not yet fully implemented for severity: %s", severity)
}

func ClearWhitelist() error {
	if IsMCPMode() {
		return errors.New("security config updates require human review - " + securityConfigEditInstruction())
	}
	if !IsInteractiveTerminal() {
		return errors.New("not in interactive mode - " + securityConfigEditInstruction())
	}
	return errors.New("ClearWhitelist not yet fully implemented")
}
