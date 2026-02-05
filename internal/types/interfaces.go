// interfaces.go — Interface definitions for cross-package dependencies.
//
// IMPORTANT: Most interfaces in this file are DEPRECATED and NOT USED.
// The ToolHandler uses concrete types directly (see tools_core.go).
//
// These interfaces were created speculatively but their signatures don't match
// the actual implementations. See interface_checks.go for documentation of the
// mismatches.
//
// RECOMMENDATION: Use concrete types directly instead of these interfaces.
// If you need polymorphism, update the interface to match the implementation.
package types

import "encoding/json"

// ============================================
// DEPRECATED INTERFACES
// ============================================
// The following interfaces are NOT used and their signatures don't match
// implementations. They are kept for backwards compatibility but should
// not be used in new code.

// CheckpointManager manages session state snapshots for before/after comparisons.
// DEPRECATED: Not implemented. Use concrete types directly.
type CheckpointManager interface {
	Capture(name string) error
	List() []string
	Get(name string) interface{}
	Delete(name string) error
	Compare(from, to string) (interface{}, error)
}

// SessionStore manages persistent session data storage.
// DEPRECATED: Signature mismatch. Use *ai.SessionStore directly.
// Interface uses (data interface{}), implementation uses (data []byte).
type SessionStore interface {
	Save(namespace, key string, data interface{}) error
	Load(namespace, key string) (interface{}, error)
	List(namespace string) ([]string, error)
	Delete(namespace, key string) error
	Stats() interface{}
}

// NoiseConfig manages noise filtering rules for telemetry.
// DEPRECATED: Signature mismatch. Use *ai.NoiseConfig directly.
// Interface has AddRule(interface{}), implementation has AddRules([]NoiseRule).
type NoiseConfig interface {
	AddRule(rule interface{}) error
	RemoveRule(ruleID string) error
	ListRules() []interface{}
	Reset()
	AutoDetect() []interface{}
	Match(entry interface{}) bool
}

// ClusterManager groups and analyzes similar errors.
// DEPRECATED: Not implemented.
type ClusterManager interface {
	Feed(entries []interface{})
	GetClusters() []interface{}
	Clear()
}

// TemporalGraph tracks temporal relationships between events.
// DEPRECATED: Not implemented.
type TemporalGraph interface {
	AddEvent(eventType string, data interface{})
	GetTimeline() []interface{}
	Clear()
}

// AlertBuffer manages pending alerts for streaming.
// DEPRECATED: Not implemented.
type AlertBuffer interface {
	Add(alert interface{})
	Drain() []interface{}
	Count() int
}

// CSPGenerator generates Content-Security-Policy headers.
// DEPRECATED: Not implemented as interface.
type CSPGenerator interface {
	Generate(mode string) (interface{}, error)
}

// SecurityScanner scans for security issues in network traffic.
// DEPRECATED: Signature mismatch. Use *security.SecurityScanner directly.
// Interface has Scan(checks, severityMin), implementation has Scan(SecurityScanInput).
type SecurityScanner interface {
	Scan(checks []string, severityMin string) (interface{}, error)
}

// ThirdPartyAuditor audits third-party resources.
// DEPRECATED: Signature mismatch. Use *analysis.ThirdPartyAuditor directly.
// Interface has Audit(origins, includeStatic), implementation has Audit(bodies, pageURLs, params).
type ThirdPartyAuditor interface {
	Audit(firstPartyOrigins []string, includeStatic bool) (interface{}, error)
}

// SecurityDiffManager compares security snapshots.
// DEPRECATED: Not implemented.
type SecurityDiffManager interface {
	Capture(name string) error
	Compare(from, to string) (interface{}, error)
	List() []string
}

// AuditTrail logs tool invocations for audit purposes.
// DEPRECATED: Not implemented.
type AuditTrail interface {
	Log(toolName string, params json.RawMessage)
	Query(sessionID, toolName, since string, limit int) []interface{}
}

// APIContractValidator validates API responses against learned contracts.
// DEPRECATED: Not implemented.
type APIContractValidator interface {
	Analyze(urlFilter string, ignoreEndpoints []string) (interface{}, error)
	Report() interface{}
	Clear()
}

// SessionManager manages browser session state.
// DEPRECATED: Not implemented as interface.
type SessionManager interface {
	CaptureSnapshot(name string) error
	CompareSnapshots(a, b string) (interface{}, error)
	ListSnapshots() []string
	DeleteSnapshot(name string) error
}
