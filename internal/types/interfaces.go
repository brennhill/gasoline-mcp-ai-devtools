// interfaces.go — Interface definitions for cross-package dependencies.
// These interfaces allow packages to depend on abstractions rather than
// concrete types, preventing circular imports.
//
// USAGE: Import this package to use interfaces for type-safe dependencies
// without creating circular imports. Implementations are in their respective
// packages (ai, analysis, security, etc.).
package types

import "encoding/json"

// ============================================
// Checkpoint Manager Interface
// ============================================

// CheckpointManager manages session state snapshots for before/after comparisons.
// Implemented by: internal/ai.CheckpointManager
type CheckpointManager interface {
	// Capture creates a named snapshot of current session state
	Capture(name string) error
	// List returns all available checkpoint names
	List() []string
	// Get returns a checkpoint by name (nil if not found)
	Get(name string) interface{}
	// Delete removes a checkpoint by name
	Delete(name string) error
	// Compare returns differences between two checkpoints
	Compare(from, to string) (interface{}, error)
}

// ============================================
// Session Store Interface
// ============================================

// SessionStore manages persistent session data storage.
// Implemented by: internal/ai.SessionStore
type SessionStore interface {
	// Save stores data under the given namespace and key
	Save(namespace, key string, data interface{}) error
	// Load retrieves data by namespace and key
	Load(namespace, key string) (interface{}, error)
	// List returns all keys in a namespace
	List(namespace string) ([]string, error)
	// Delete removes data by namespace and key
	Delete(namespace, key string) error
	// Stats returns storage statistics
	Stats() interface{}
}

// ============================================
// Noise Configuration Interface
// ============================================

// NoiseConfig manages noise filtering rules for telemetry.
// Implemented by: internal/ai.NoiseConfig
type NoiseConfig interface {
	// AddRule adds a noise filtering rule
	AddRule(rule interface{}) error
	// RemoveRule removes a rule by ID
	RemoveRule(ruleID string) error
	// ListRules returns all configured rules
	ListRules() []interface{}
	// Reset clears all rules
	Reset()
	// AutoDetect suggests rules based on observed patterns
	AutoDetect() []interface{}
	// Match checks if an entry matches any noise rule
	Match(entry interface{}) bool
}

// ============================================
// Cluster Manager Interface
// ============================================

// ClusterManager groups and analyzes similar errors.
// Implemented by: internal/analysis.ClusterManager
type ClusterManager interface {
	// Feed processes new entries for clustering
	Feed(entries []interface{})
	// GetClusters returns current error clusters
	GetClusters() []interface{}
	// Clear resets all clusters
	Clear()
}

// ============================================
// Temporal Graph Interface
// ============================================

// TemporalGraph tracks temporal relationships between events.
// Implemented by: internal/codegen.TemporalGraph
type TemporalGraph interface {
	// AddEvent records an event with timestamp
	AddEvent(eventType string, data interface{})
	// GetTimeline returns events in chronological order
	GetTimeline() []interface{}
	// Clear resets the graph
	Clear()
}

// ============================================
// Alert Buffer Interface
// ============================================

// AlertBuffer manages pending alerts for streaming.
// Implemented by: internal/ai.AlertBuffer
type AlertBuffer interface {
	// Add adds an alert to the buffer
	Add(alert interface{})
	// Drain returns and clears all pending alerts
	Drain() []interface{}
	// Count returns the number of pending alerts
	Count() int
}

// ============================================
// Security Tool Interfaces
// ============================================

// CSPGenerator generates Content-Security-Policy headers.
// Implemented by: internal/security.CSPGenerator
type CSPGenerator interface {
	// Generate creates a CSP policy from observed network data
	Generate(mode string) (interface{}, error)
}

// SecurityScanner scans for security issues in network traffic.
// Implemented by: internal/security.SecurityScanner
type SecurityScanner interface {
	// Scan performs a security audit with the given checks
	Scan(checks []string, severityMin string) (interface{}, error)
}

// ThirdPartyAuditor audits third-party resources.
// Implemented by: internal/security.ThirdPartyAuditor
type ThirdPartyAuditor interface {
	// Audit analyzes third-party origins
	Audit(firstPartyOrigins []string, includeStatic bool) (interface{}, error)
}

// SecurityDiffManager compares security snapshots.
// Implemented by: internal/security.SecurityDiffManager
type SecurityDiffManager interface {
	// Capture creates a security snapshot
	Capture(name string) error
	// Compare compares two snapshots
	Compare(from, to string) (interface{}, error)
	// List returns available snapshot names
	List() []string
}

// ============================================
// Audit Trail Interface
// ============================================

// AuditTrail logs tool invocations for audit purposes.
// Implemented by: internal/audit.AuditTrail
type AuditTrail interface {
	// Log records a tool invocation
	Log(toolName string, params json.RawMessage)
	// Query returns audit entries matching criteria
	Query(sessionID, toolName, since string, limit int) []interface{}
}

// ============================================
// API Contract Validator Interface
// ============================================

// APIContractValidator validates API responses against learned contracts.
// Implemented by: internal/analysis.APIContractValidator
type APIContractValidator interface {
	// Analyze checks for contract violations
	Analyze(urlFilter string, ignoreEndpoints []string) (interface{}, error)
	// Report returns the learned contract summary
	Report() interface{}
	// Clear resets learned contracts
	Clear()
}

// ============================================
// Session Manager Interface
// ============================================

// SessionManager manages browser session state.
// Implemented by: internal/session.SessionManager
type SessionManager interface {
	// CaptureSnapshot creates a state snapshot
	CaptureSnapshot(name string) error
	// CompareSnapshots compares two snapshots
	CompareSnapshots(a, b string) (interface{}, error)
	// ListSnapshots returns available snapshot names
	ListSnapshots() []string
	// DeleteSnapshot removes a snapshot
	DeleteSnapshot(name string) error
}
