// audit_trail.go — Enterprise Audit Trail (Tier 1).
// Append-only tool invocation log with client identification, session management,
// parameter redaction, and redaction event logging.
// Design: The AuditTrail struct is a standalone, concurrent-safe, bounded buffer
// that records every MCP tool call. Entries are never modified or deleted — only
// evicted via FIFO when the buffer is full. The log is queryable with filters
// for session, tool name, and time range.
package audit

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================
// Types
// ============================================

// AuditEntry represents a single tool invocation audit record.
type AuditEntry struct {
	ID           string    `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	SessionID    string    `json:"session_id"`
	ClientID     string    `json:"client_id"`
	ToolName     string    `json:"tool_name"`
	Parameters   string    `json:"parameters"`
	ResponseSize int       `json:"response_size"`
	Duration     int64     `json:"duration_ms"`
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// AuditTrail is an append-only, bounded, concurrent-safe audit log.
type AuditTrail struct {
	mu              sync.RWMutex
	entries         []AuditEntry
	maxSize         int
	sessions        map[string]*SessionInfo
	config          AuditConfig
	redactions      []RedactionEvent
	redactionPatterns []*redactionPattern
}

// SessionInfo tracks metadata for an MCP session.
type SessionInfo struct {
	ID        string    `json:"id"`
	ClientID  string    `json:"client_id"`
	StartedAt time.Time `json:"started_at"`
	ToolCalls int       `json:"tool_calls"`
}

// AuditFilter specifies query criteria for audit log entries.
type AuditFilter struct {
	SessionID string     `json:"session_id,omitempty"`
	ToolName  string     `json:"tool_name,omitempty"`
	Since     *time.Time `json:"since,omitempty"`
	Limit     int        `json:"limit,omitempty"`
}

// AuditConfig controls audit trail behavior.
type AuditConfig struct {
	MaxEntries   int  `json:"max_entries"`
	Enabled      bool `json:"enabled"`
	RedactParams bool `json:"redact_params"`
}

// ClientIdentifier holds MCP client identification from the initialize message.
type ClientIdentifier struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// RedactionEvent records that a redaction pattern matched, without storing content.
type RedactionEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	SessionID   string    `json:"session_id"`
	ToolName    string    `json:"tool_name"`
	FieldPath   string    `json:"field_path"`
	PatternName string    `json:"pattern_name"`
}

// redactionPattern is an internal compiled regex for parameter redaction.
type redactionPattern struct {
	name    string
	pattern *regexp.Regexp
}

// ============================================
// Constants
// ============================================

const (
	defaultAuditMaxEntries = 10000
	defaultAuditQueryLimit = 100
)

// ============================================
// Constructor
// ============================================

// NewAuditTrail creates a new audit trail with the given configuration.
// Zero-value AuditConfig fields are replaced with sensible defaults.
func NewAuditTrail(config AuditConfig) *AuditTrail {
	if config.MaxEntries <= 0 {
		config.MaxEntries = defaultAuditMaxEntries
	}
	// Default: enabled and redact params when config is zero-value
	if !config.Enabled && config.MaxEntries == defaultAuditMaxEntries && !config.RedactParams {
		config.Enabled = true
		config.RedactParams = true
	}

	trail := &AuditTrail{
		entries:  make([]AuditEntry, 0, config.MaxEntries),
		maxSize:  config.MaxEntries,
		sessions: make(map[string]*SessionInfo),
		config:   config,
		redactions: make([]RedactionEvent, 0),
	}

	if config.RedactParams {
		trail.redactionPatterns = compileRedactionPatterns()
	}

	return trail
}

// ============================================
// Recording
// ============================================

// Record appends an audit entry to the log. If the trail is disabled, the
// entry is silently dropped. If the buffer is full, the oldest entry is evicted.
func (at *AuditTrail) Record(entry AuditEntry) {
	if !at.config.Enabled {
		return
	}

	at.mu.Lock()
	defer at.mu.Unlock()

	// Assign ID and timestamp
	entry.ID = generateAuditID()
	entry.Timestamp = time.Now()

	// Redact parameters if configured
	if at.config.RedactParams && entry.Parameters != "" {
		entry.Parameters = at.redactParameters(entry.Parameters)
	}

	// FIFO eviction when full
	if len(at.entries) >= at.maxSize {
		newEntries := make([]AuditEntry, len(at.entries)-1)
		copy(newEntries, at.entries[1:])
		at.entries = newEntries
	}

	at.entries = append(at.entries, entry)

	// Update session tool call count
	if sess, ok := at.sessions[entry.SessionID]; ok {
		sess.ToolCalls++
	}
}

// RecordRedaction logs a redaction event without storing any redacted content.
func (at *AuditTrail) RecordRedaction(event RedactionEvent) {
	if !at.config.Enabled {
		return
	}

	at.mu.Lock()
	defer at.mu.Unlock()

	// Bounded: use same max size for redaction events
	if len(at.redactions) >= at.maxSize {
		newRedactions := make([]RedactionEvent, len(at.redactions)-1)
		copy(newRedactions, at.redactions[1:])
		at.redactions = newRedactions
	}

	at.redactions = append(at.redactions, event)
}

// ============================================
// Querying
// ============================================

// Query returns audit entries matching the given filter.
// Results are returned in reverse chronological order (newest first).
// If no limit is specified, defaults to 100.
func (at *AuditTrail) Query(filter AuditFilter) []AuditEntry {
	at.mu.RLock()
	defer at.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultAuditQueryLimit
	}

	// Walk entries in reverse order (newest first)
	var results []AuditEntry
	for i := len(at.entries) - 1; i >= 0 && len(results) < limit; i-- {
		e := at.entries[i]

		if filter.SessionID != "" && e.SessionID != filter.SessionID {
			continue
		}
		if filter.ToolName != "" && e.ToolName != filter.ToolName {
			continue
		}
		if filter.Since != nil && e.Timestamp.Before(*filter.Since) {
			continue
		}

		results = append(results, e)
	}

	return results
}

// QueryRedactions returns redaction events matching the given filter.
func (at *AuditTrail) QueryRedactions(filter AuditFilter) []RedactionEvent {
	at.mu.RLock()
	defer at.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultAuditQueryLimit
	}

	var results []RedactionEvent
	for i := len(at.redactions) - 1; i >= 0 && len(results) < limit; i-- {
		e := at.redactions[i]

		if filter.SessionID != "" && e.SessionID != filter.SessionID {
			continue
		}
		if filter.ToolName != "" && e.ToolName != filter.ToolName {
			continue
		}

		results = append(results, e)
	}

	return results
}

// ============================================
// Client Identification
// ============================================

// IdentifyClient normalizes the client name from the MCP initialize message.
// Known clients are lowercased; unknown names are preserved as-is.
func (at *AuditTrail) IdentifyClient(client ClientIdentifier) string {
	if client.Name == "" {
		return "unknown"
	}

	lower := strings.ToLower(client.Name)

	// Known clients — normalize to lowercase
	switch lower {
	case "claude-code", "cursor", "windsurf", "cline":
		return lower
	}

	// Unknown client — preserve raw name
	return client.Name
}

// ============================================
// Session Management
// ============================================

// CreateSession generates a new unique session and registers it.
// The session ID is a 16-byte random value, hex-encoded (32 chars).
func (at *AuditTrail) CreateSession(client ClientIdentifier) *SessionInfo {
	at.mu.Lock()
	defer at.mu.Unlock()

	sessionID := generateSessionID()
	clientID := at.identifyClientUnlocked(client)

	info := &SessionInfo{
		ID:        sessionID,
		ClientID:  clientID,
		StartedAt: time.Now(),
		ToolCalls: 0,
	}

	at.sessions[sessionID] = info
	return info
}

// GetSession returns session info for the given ID, or nil if not found.
func (at *AuditTrail) GetSession(id string) *SessionInfo {
	at.mu.RLock()
	defer at.mu.RUnlock()

	return at.sessions[id]
}

// identifyClientUnlocked is the internal version without locking.
func (at *AuditTrail) identifyClientUnlocked(client ClientIdentifier) string {
	if client.Name == "" {
		return "unknown"
	}

	lower := strings.ToLower(client.Name)
	switch lower {
	case "claude-code", "cursor", "windsurf", "cline":
		return lower
	}

	return client.Name
}

// ============================================
// MCP Tool Handler
// ============================================

// HandleGetAuditLog is the MCP tool handler for get_audit_log.
// It parses filter parameters and returns matching audit entries.
func (at *AuditTrail) HandleGetAuditLog(params json.RawMessage) (any, error) {
	var filter AuditFilter
	if len(params) > 0 && string(params) != "{}" {
		if err := json.Unmarshal(params, &filter); err != nil {
			return nil, err
		}
	}

	entries := at.Query(filter)

	type auditLogResponse struct {
		Entries []AuditEntry `json:"entries"`
		Count   int          `json:"count"`
	}

	return auditLogResponse{
		Entries: entries,
		Count:   len(entries),
	}, nil
}

// ============================================
// Parameter Redaction
// ============================================

// redactParameters applies all configured redaction patterns to the parameter string.
func (at *AuditTrail) redactParameters(params string) string {
	result := params
	for _, rp := range at.redactionPatterns {
		result = rp.pattern.ReplaceAllString(result, "[REDACTED]")
	}
	return result
}

// compileRedactionPatterns returns the built-in redaction patterns compiled.
func compileRedactionPatterns() []*redactionPattern {
	patterns := []struct {
		name    string
		pattern string
	}{
		// Bearer tokens (OAuth)
		{"bearer_token", `Bearer\s+[A-Za-z0-9\-._~+/]+=*`},
		// API keys in key=value format
		{"api_key", `(?i)(api[_-]?key|apikey|secret[_-]?key)\s*[:=]\s*\S+`},
		// JSON Web Tokens
		{"jwt", `eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*`},
		// Session cookies/tokens (long random values)
		{"session_cookie", `(?i)(session|sid|token)\s*=\s*[A-Za-z0-9+/=_-]{16,}`},
	}

	compiled := make([]*redactionPattern, 0, len(patterns))
	for _, p := range patterns {
		re := regexp.MustCompile(p.pattern)
		compiled = append(compiled, &redactionPattern{
			name:    p.name,
			pattern: re,
		})
	}

	return compiled
}

// ============================================
// ID Generation
// ============================================

// generateAuditID creates a unique audit entry ID (16 hex chars).
func generateAuditID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b) // #nosec G104 -- best-effort randomness for non-security audit ID
	return hex.EncodeToString(b)
}

// generateSessionID creates a unique session ID (32 hex chars from 16 random bytes).
func generateSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // #nosec G104 -- best-effort randomness for non-security session ID
	return hex.EncodeToString(b)
}
