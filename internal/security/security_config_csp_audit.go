// Purpose: Defines the CSP audit structure for tracking session overrides and persistent whitelists.
// Why: Isolates CSP audit types from the main security configuration.
package security

type CSPAudit struct {
	SessionOverrides    []string `json:"session_overrides,omitempty"`
	PersistentWhitelist []string `json:"persistent_whitelist,omitempty"`
	OverrideSource      string   `json:"override_source,omitempty"`
}
