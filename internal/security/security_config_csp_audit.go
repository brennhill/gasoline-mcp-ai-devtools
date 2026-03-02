package security

type CSPAudit struct {
	SessionOverrides    []string `json:"session_overrides,omitempty"`
	PersistentWhitelist []string `json:"persistent_whitelist,omitempty"`
	OverrideSource      string   `json:"override_source,omitempty"`
}
