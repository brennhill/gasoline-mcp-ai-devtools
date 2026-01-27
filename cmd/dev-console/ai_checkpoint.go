// ai_checkpoint.go — Named checkpoint/diff system for session state comparison.
// Checkpoints record buffer positions at a point in time. Diffs return only
// new entries since the checkpoint, deduplicated and severity-filtered.
// Design: Checkpoints store position counters, not data copies, making them
// cheap to create. Max 20 named checkpoints with LRU eviction. Auto-checkpoint
// advances on each diff call for "show me what's new" workflows.
package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// ============================================
// Constants
// ============================================

const (
	maxNamedCheckpoints   = 20
	maxCheckpointNameLen  = 50
	maxDiffEntriesPerCat  = 50
	maxMessageLen         = 200
	degradedLatencyFactor = 3
)

// ============================================
// Types
// ============================================

// Checkpoint stores a snapshot of buffer positions at a point in time
type Checkpoint struct {
	Name           string
	CreatedAt      time.Time
	LogTotal       int64
	NetworkTotal   int64
	WSTotal        int64
	ActionTotal    int64
	PageURL        string
	KnownEndpoints map[string]endpointState // path → state
	AlertDelivery  int64                    // alert delivery counter at checkpoint time
}

// endpointState tracks the last known state of an endpoint
type endpointState struct {
	Status   int
	Duration int // baseline latency in ms
}

// GetChangesSinceParams defines the parameters for get_changes_since
type GetChangesSinceParams struct {
	Checkpoint string   // named checkpoint, ISO timestamp, or empty for auto
	Include    []string // "console", "network", "websocket", "actions"
	Severity   string   // "all", "warnings", "errors_only"
}

// DiffResponse is the compressed diff returned by get_changes_since
type DiffResponse struct {
	From              time.Time          `json:"from"`
	To                time.Time          `json:"to"`
	DurationMs        int64              `json:"duration_ms"`
	Severity          string             `json:"severity"`
	Summary           string             `json:"summary"`
	TokenCount        int                `json:"token_count"`
	Console           *ConsoleDiff       `json:"console,omitempty"`
	Network           *NetworkDiff       `json:"network,omitempty"`
	WebSocket         *WebSocketDiff     `json:"websocket,omitempty"`
	Actions           *ActionsDiff       `json:"actions,omitempty"`
	PerformanceAlerts []PerformanceAlert `json:"performance_alerts,omitempty"`
}

// ConsoleDiff contains deduplicated console entries since the checkpoint
type ConsoleDiff struct {
	TotalNew int            `json:"total_new"`
	Errors   []ConsoleEntry `json:"errors,omitempty"`
	Warnings []ConsoleEntry `json:"warnings,omitempty"`
}

// ConsoleEntry is a deduplicated console message
type ConsoleEntry struct {
	Message string `json:"message"`
	Source  string `json:"source,omitempty"`
	Count   int    `json:"count"`
}

// NetworkDiff contains network changes since the checkpoint
type NetworkDiff struct {
	TotalNew     int               `json:"total_new"`
	Failures     []NetworkFailure  `json:"failures,omitempty"`
	NewEndpoints []string          `json:"new_endpoints,omitempty"`
	Degraded     []NetworkDegraded `json:"degraded,omitempty"`
}

// NetworkFailure represents an endpoint that started failing
type NetworkFailure struct {
	Path           string `json:"path"`
	Status         int    `json:"status"`
	PreviousStatus int    `json:"previous_status"`
}

// NetworkDegraded represents an endpoint with degraded latency
type NetworkDegraded struct {
	Path     string `json:"path"`
	Duration int    `json:"duration_ms"`
	Baseline int    `json:"baseline_ms"`
}

// WebSocketDiff contains WebSocket changes since the checkpoint
type WebSocketDiff struct {
	TotalNew       int       `json:"total_new"`
	Disconnections []WSDisco `json:"disconnections,omitempty"`
	Connections    []WSConn  `json:"connections,omitempty"`
	Errors         []WSError `json:"errors,omitempty"`
}

// WSDisco represents a WebSocket disconnection
type WSDisco struct {
	URL         string `json:"url"`
	CloseCode   int    `json:"close_code,omitempty"`
	CloseReason string `json:"close_reason,omitempty"`
}

// WSConn represents a new WebSocket connection
type WSConn struct {
	URL string `json:"url"`
	ID  string `json:"id"`
}

// WSError represents a WebSocket error event
type WSError struct {
	URL     string `json:"url"`
	Message string `json:"message"`
}

// ActionsDiff contains user actions since the checkpoint
type ActionsDiff struct {
	TotalNew int           `json:"total_new"`
	Actions  []ActionEntry `json:"actions,omitempty"`
}

// ActionEntry represents a user action in the diff
type ActionEntry struct {
	Type      string `json:"type"`
	URL       string `json:"url,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// CheckpointManager manages checkpoints and computes diffs
type CheckpointManager struct {
	mu               sync.Mutex
	autoCheckpoint   *Checkpoint
	namedCheckpoints map[string]*Checkpoint
	namedOrder       []string // track insertion order for eviction

	server *Server

	// Push regression alerts
	pendingAlerts []PerformanceAlert
	alertCounter  int64
	alertDelivery int64 // monotonic counter for delivery tracking
	capture       *Capture
}

// ============================================
// Constructor
// ============================================

// NewCheckpointManager creates a new checkpoint manager
func NewCheckpointManager(server *Server, capture *Capture) *CheckpointManager {
	return &CheckpointManager{
		namedCheckpoints: make(map[string]*Checkpoint),
		namedOrder:       make([]string, 0),
		server:           server,
		capture:          capture,
	}
}

// ============================================
// Public API
// ============================================

// CreateCheckpoint creates a named checkpoint at the current buffer positions.
// If clientID is provided, the checkpoint is namespaced as "clientID:name".
// This enables multi-client isolation where each client has its own checkpoint space.
func (cm *CheckpointManager) CreateCheckpoint(name string, clientID string) error {
	if name == "" {
		return fmt.Errorf("checkpoint name cannot be empty")
	}
	if len(name) > maxCheckpointNameLen {
		return fmt.Errorf("checkpoint name exceeds %d characters", maxCheckpointNameLen)
	}

	// Namespace the checkpoint name with client ID
	storedName := name
	if clientID != "" {
		storedName = clientID + ":" + name
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cp := cm.snapshotNow()
	cp.Name = name // Store the original name, not the namespaced one
	cp.AlertDelivery = cm.alertDelivery

	// If name already exists, update it
	if _, exists := cm.namedCheckpoints[storedName]; !exists {
		cm.namedOrder = append(cm.namedOrder, storedName)
	}
	cm.namedCheckpoints[storedName] = cp

	// Enforce max named checkpoints (evict oldest)
	for len(cm.namedCheckpoints) > maxNamedCheckpoints {
		oldest := cm.namedOrder[0]
		newOrder := make([]string, len(cm.namedOrder)-1)
		copy(newOrder, cm.namedOrder[1:])
		cm.namedOrder = newOrder
		delete(cm.namedCheckpoints, oldest)
	}

	return nil
}

// GetNamedCheckpointCount returns the number of named checkpoints
func (cm *CheckpointManager) GetNamedCheckpointCount() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return len(cm.namedCheckpoints)
}

// GetChangesSince computes a compressed diff since the specified checkpoint.
// If clientID is provided, checkpoint names are resolved with the client prefix.
func (cm *CheckpointManager) GetChangesSince(params GetChangesSinceParams, clientID string) DiffResponse {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	isNamedQuery := false

	// Build the namespaced checkpoint name for lookup
	namespacedCheckpoint := params.Checkpoint
	if params.Checkpoint != "" && clientID != "" {
		namespacedCheckpoint = clientID + ":" + params.Checkpoint
	}

	// Resolve checkpoint
	var cp *Checkpoint
	if params.Checkpoint == "" {
		// Auto-checkpoint mode
		if cm.autoCheckpoint == nil {
			// First call: create checkpoint at buffer start
			cp = &Checkpoint{
				CreatedAt:      now,
				LogTotal:       0,
				NetworkTotal:   0,
				WSTotal:        0,
				ActionTotal:    0,
				KnownEndpoints: make(map[string]endpointState),
			}
		} else {
			cp = cm.autoCheckpoint
		}
	} else if named, ok := cm.namedCheckpoints[namespacedCheckpoint]; ok {
		// Named checkpoint (with client namespace)
		cp = named
		isNamedQuery = true
	} else if named, ok := cm.namedCheckpoints[params.Checkpoint]; ok {
		// Fall back to global checkpoint (backwards compatibility)
		cp = named
		isNamedQuery = true
	} else {
		// Try parsing as timestamp
		cp = cm.resolveTimestampCheckpoint(params.Checkpoint)
		if cp == nil {
			// Unknown checkpoint, treat as beginning
			cp = &Checkpoint{
				CreatedAt:      now,
				KnownEndpoints: make(map[string]endpointState),
			}
		}
		isNamedQuery = true // timestamp queries don't advance auto-checkpoint
	}

	// Compute diffs
	resp := DiffResponse{
		From: cp.CreatedAt,
		To:   now,
	}
	resp.DurationMs = now.Sub(cp.CreatedAt).Milliseconds()

	// Determine which categories to include
	includeConsole := cm.shouldInclude(params.Include, "console")
	includeNetwork := cm.shouldInclude(params.Include, "network")
	includeWS := cm.shouldInclude(params.Include, "websocket")
	includeActions := cm.shouldInclude(params.Include, "actions")

	// Compute each category's diff
	if includeConsole {
		resp.Console = cm.computeConsoleDiff(cp, params.Severity)
	}
	if includeNetwork {
		resp.Network = cm.computeNetworkDiff(cp)
	}
	if includeWS {
		resp.WebSocket = cm.computeWebSocketDiff(cp, params.Severity)
	}
	if includeActions {
		resp.Actions = cm.computeActionsDiff(cp)
	}

	// Apply severity filter to overall response
	if params.Severity == "errors_only" {
		// Remove warning-level categories
		if resp.Console != nil {
			resp.Console.Warnings = nil
		}
		// WebSocket: only keep if there are actual errors (not just disconnections/connections)
		if resp.WebSocket != nil && len(resp.WebSocket.Errors) == 0 {
			resp.WebSocket = nil
		}
	}

	// Determine overall severity
	resp.Severity = cm.determineSeverity(resp)

	// Build summary
	resp.Summary = cm.buildSummary(resp)

	// Nil out empty diffs
	if resp.Console != nil && resp.Console.TotalNew == 0 {
		resp.Console = nil
	}
	if resp.Network != nil && resp.Network.TotalNew == 0 && len(resp.Network.Failures) == 0 && len(resp.Network.NewEndpoints) == 0 && len(resp.Network.Degraded) == 0 {
		resp.Network = nil
	}
	if resp.WebSocket != nil && resp.WebSocket.TotalNew == 0 {
		resp.WebSocket = nil
	}
	if resp.Actions != nil && resp.Actions.TotalNew == 0 {
		resp.Actions = nil
	}

	// Include performance alerts
	alerts := cm.getPendingAlerts(cp.AlertDelivery)
	if len(alerts) > 0 {
		resp.PerformanceAlerts = alerts
	}

	// Calculate token count
	// Error impossible: result structure contains only serializable types
	jsonBytes, _ := json.Marshal(resp)
	resp.TokenCount = len(jsonBytes) / 4

	// Advance auto-checkpoint (only for auto-mode queries)
	if !isNamedQuery {
		// Mark alerts as delivered so they will not appear on next poll
		cm.markAlertsDelivered()
		cm.autoCheckpoint = cm.snapshotNow()
		// Update known endpoints from current network state
		cm.autoCheckpoint.KnownEndpoints = cm.buildKnownEndpoints(cp.KnownEndpoints)
		cm.autoCheckpoint.AlertDelivery = cm.alertDelivery
	}

	return resp
}

// ============================================
// Internal: Checkpoint resolution
// ============================================

// snapshotNow reads buffer positions under two separate locks (server.mu, capture.mu).
// This is not globally atomic: a log entry could arrive between the two reads. This is
// acceptable because checkpoints are advisory positions for "show me what changed" diffs,
// and a ±1 entry variance has no correctness impact.
func (cm *CheckpointManager) snapshotNow() *Checkpoint {
	cm.server.mu.RLock()
	logTotal := cm.server.logTotalAdded
	cm.server.mu.RUnlock()

	cm.capture.mu.RLock()
	netTotal := cm.capture.networkTotalAdded
	wsTotal := cm.capture.wsTotalAdded
	actTotal := cm.capture.actionTotalAdded
	cm.capture.mu.RUnlock()

	return &Checkpoint{
		CreatedAt:      time.Now(),
		LogTotal:       logTotal,
		NetworkTotal:   netTotal,
		WSTotal:        wsTotal,
		ActionTotal:    actTotal,
		KnownEndpoints: make(map[string]endpointState),
	}
}

func (cm *CheckpointManager) resolveTimestampCheckpoint(tsStr string) *Checkpoint {
	t, err := time.Parse(time.RFC3339Nano, tsStr)
	if err != nil {
		t, err = time.Parse(time.RFC3339, tsStr)
		if err != nil {
			return nil
		}
	}

	// Find buffer positions at the given timestamp using addedAt slices
	cm.server.mu.RLock()
	logTotal := cm.findPositionAtTime(cm.server.logAddedAt, cm.server.logTotalAdded, t)
	cm.server.mu.RUnlock()

	cm.capture.mu.RLock()
	netTotal := cm.findPositionAtTime(cm.capture.networkAddedAt, cm.capture.networkTotalAdded, t)
	wsTotal := cm.findPositionAtTime(cm.capture.wsAddedAt, cm.capture.wsTotalAdded, t)
	actTotal := cm.findPositionAtTime(cm.capture.actionAddedAt, cm.capture.actionTotalAdded, t)
	cm.capture.mu.RUnlock()

	return &Checkpoint{
		CreatedAt:      t,
		LogTotal:       logTotal,
		NetworkTotal:   netTotal,
		WSTotal:        wsTotal,
		ActionTotal:    actTotal,
		KnownEndpoints: make(map[string]endpointState),
	}
}

// findPositionAtTime finds the totalAdded count at a given timestamp
// using the parallel addedAt slice and binary search.
func (cm *CheckpointManager) findPositionAtTime(addedAt []time.Time, currentTotal int64, t time.Time) int64 {
	if len(addedAt) == 0 {
		return currentTotal
	}

	// Binary search for the first entry added after time t
	idx := sort.Search(len(addedAt), func(i int) bool {
		return addedAt[i].After(t)
	})

	// idx is the number of entries that were added at or before time t
	// The total at that point is: currentTotal - (len(addedAt) - idx)
	entriesAfter := int64(len(addedAt) - idx)
	pos := currentTotal - entriesAfter
	if pos < 0 {
		pos = 0
	}
	return pos
}

func (cm *CheckpointManager) shouldInclude(include []string, category string) bool {
	if len(include) == 0 {
		return true // default: include all
	}
	for _, inc := range include {
		if inc == category {
			return true
		}
	}
	return false
}

// ============================================
// Internal: Diff computation
// ============================================

func (cm *CheckpointManager) computeConsoleDiff(cp *Checkpoint, severity string) *ConsoleDiff {
	cm.server.mu.RLock()
	currentTotal := cm.server.logTotalAdded
	newCount := int(currentTotal - cp.LogTotal)
	if newCount <= 0 {
		cm.server.mu.RUnlock()
		return &ConsoleDiff{}
	}
	available := len(cm.server.entries)
	toRead := newCount
	if toRead > available {
		toRead = available
	}
	// Copy the slice subset under the lock to avoid data race on backing array
	newEntries := make([]LogEntry, toRead)
	copy(newEntries, cm.server.entries[available-toRead:])
	cm.server.mu.RUnlock()

	// Separate into errors and warnings, deduplicate by fingerprint
	type fingerprintEntry struct {
		message string
		source  string
		count   int
	}
	errorMap := make(map[string]*fingerprintEntry)
	warningMap := make(map[string]*fingerprintEntry)
	var errorOrder, warningOrder []string

	totalNew := 0
	for _, entry := range newEntries {
		level, _ := entry["level"].(string)
		msg, _ := entry["msg"].(string)
		if msg == "" {
			msg, _ = entry["message"].(string)
		}
		source, _ := entry["source"].(string)

		if level == "error" {
			totalNew++
			fp := FingerprintMessage(msg)
			if existing, ok := errorMap[fp]; ok {
				existing.count++
			} else {
				truncMsg := truncateMessage(msg)
				errorMap[fp] = &fingerprintEntry{message: truncMsg, source: source, count: 1}
				errorOrder = append(errorOrder, fp)
			}
		} else if level == "warn" || level == "warning" {
			totalNew++
			if severity == "errors_only" {
				continue
			}
			fp := FingerprintMessage(msg)
			if existing, ok := warningMap[fp]; ok {
				existing.count++
			} else {
				truncMsg := truncateMessage(msg)
				warningMap[fp] = &fingerprintEntry{message: truncMsg, source: source, count: 1}
				warningOrder = append(warningOrder, fp)
			}
		} else {
			totalNew++
		}
	}

	diff := &ConsoleDiff{TotalNew: totalNew}

	// Build error entries (capped at max)
	for i, fp := range errorOrder {
		if i >= maxDiffEntriesPerCat {
			break
		}
		e := errorMap[fp]
		diff.Errors = append(diff.Errors, ConsoleEntry{
			Message: e.message,
			Source:  e.source,
			Count:   e.count,
		})
	}

	// Build warning entries (capped at max)
	for i, fp := range warningOrder {
		if i >= maxDiffEntriesPerCat {
			break
		}
		w := warningMap[fp]
		diff.Warnings = append(diff.Warnings, ConsoleEntry{
			Message: w.message,
			Source:  w.source,
			Count:   w.count,
		})
	}

	return diff
}

func (cm *CheckpointManager) computeNetworkDiff(cp *Checkpoint) *NetworkDiff {
	cm.capture.mu.RLock()
	currentTotal := cm.capture.networkTotalAdded
	newCount := int(currentTotal - cp.NetworkTotal)
	if newCount <= 0 {
		cm.capture.mu.RUnlock()
		return &NetworkDiff{}
	}
	available := len(cm.capture.networkBodies)
	toRead := newCount
	if toRead > available {
		toRead = available
	}
	// Copy under lock to avoid data race on backing array
	newBodies := make([]NetworkBody, toRead)
	copy(newBodies, cm.capture.networkBodies[available-toRead:])
	cm.capture.mu.RUnlock()

	diff := &NetworkDiff{TotalNew: len(newBodies)}

	// Track endpoints seen in new entries
	for _, body := range newBodies {
		path := extractURLPath(body.URL)

		// Check for failures (4xx/5xx where previously success)
		if body.Status >= 400 {
			if prev, known := cp.KnownEndpoints[path]; known && prev.Status < 400 {
				diff.Failures = append(diff.Failures, NetworkFailure{
					Path:           path,
					Status:         body.Status,
					PreviousStatus: prev.Status,
				})
			} else if !known {
				// New endpoint that immediately fails — count as new endpoint
				if !containsString(diff.NewEndpoints, path) {
					diff.NewEndpoints = append(diff.NewEndpoints, path)
				}
			}
		} else {
			// Check for new endpoints
			if _, known := cp.KnownEndpoints[path]; !known {
				if !containsString(diff.NewEndpoints, path) {
					diff.NewEndpoints = append(diff.NewEndpoints, path)
				}
			}

			// Check for degraded latency
			if body.Duration > 0 {
				if prev, known := cp.KnownEndpoints[path]; known && prev.Duration > 0 {
					if body.Duration > prev.Duration*degradedLatencyFactor {
						diff.Degraded = append(diff.Degraded, NetworkDegraded{
							Path:     path,
							Duration: body.Duration,
							Baseline: prev.Duration,
						})
					}
				}
			}
		}
	}

	// Cap entries
	if len(diff.Failures) > maxDiffEntriesPerCat {
		diff.Failures = diff.Failures[:maxDiffEntriesPerCat]
	}
	if len(diff.NewEndpoints) > maxDiffEntriesPerCat {
		diff.NewEndpoints = diff.NewEndpoints[:maxDiffEntriesPerCat]
	}
	if len(diff.Degraded) > maxDiffEntriesPerCat {
		diff.Degraded = diff.Degraded[:maxDiffEntriesPerCat]
	}

	return diff
}

func (cm *CheckpointManager) computeWebSocketDiff(cp *Checkpoint, severity string) *WebSocketDiff {
	cm.capture.mu.RLock()
	currentTotal := cm.capture.wsTotalAdded
	newCount := int(currentTotal - cp.WSTotal)
	if newCount <= 0 {
		cm.capture.mu.RUnlock()
		return &WebSocketDiff{}
	}
	available := len(cm.capture.wsEvents)
	toRead := newCount
	if toRead > available {
		toRead = available
	}
	// Copy under lock to avoid data race on backing array
	newEvents := make([]WebSocketEvent, toRead)
	copy(newEvents, cm.capture.wsEvents[available-toRead:])
	cm.capture.mu.RUnlock()

	diff := &WebSocketDiff{TotalNew: len(newEvents)}

	for i := range newEvents {
		switch newEvents[i].Event {
		case "close":
			if severity != "errors_only" {
				diff.Disconnections = append(diff.Disconnections, WSDisco{
					URL:         newEvents[i].URL,
					CloseCode:   newEvents[i].CloseCode,
					CloseReason: newEvents[i].CloseReason,
				})
			}
		case "open":
			diff.Connections = append(diff.Connections, WSConn{
				URL: newEvents[i].URL,
				ID:  newEvents[i].ID,
			})
		case "error":
			diff.Errors = append(diff.Errors, WSError{
				URL:     newEvents[i].URL,
				Message: newEvents[i].Data,
			})
		}
	}

	// Cap entries
	if len(diff.Disconnections) > maxDiffEntriesPerCat {
		diff.Disconnections = diff.Disconnections[:maxDiffEntriesPerCat]
	}
	if len(diff.Connections) > maxDiffEntriesPerCat {
		diff.Connections = diff.Connections[:maxDiffEntriesPerCat]
	}
	if len(diff.Errors) > maxDiffEntriesPerCat {
		diff.Errors = diff.Errors[:maxDiffEntriesPerCat]
	}

	return diff
}

func (cm *CheckpointManager) computeActionsDiff(cp *Checkpoint) *ActionsDiff {
	cm.capture.mu.RLock()
	currentTotal := cm.capture.actionTotalAdded
	newCount := int(currentTotal - cp.ActionTotal)
	if newCount <= 0 {
		cm.capture.mu.RUnlock()
		return &ActionsDiff{}
	}
	available := len(cm.capture.enhancedActions)
	toRead := newCount
	if toRead > available {
		toRead = available
	}
	// Copy under lock to avoid data race on backing array
	newActions := make([]EnhancedAction, toRead)
	copy(newActions, cm.capture.enhancedActions[available-toRead:])
	cm.capture.mu.RUnlock()

	diff := &ActionsDiff{TotalNew: len(newActions)}

	for i := range newActions {
		if i >= maxDiffEntriesPerCat {
			break
		}
		diff.Actions = append(diff.Actions, ActionEntry{
			Type:      newActions[i].Type,
			URL:       newActions[i].URL,
			Timestamp: newActions[i].Timestamp,
		})
	}

	return diff
}

// ============================================
// Internal: Severity and summary
// ============================================

func (cm *CheckpointManager) determineSeverity(resp DiffResponse) string {
	// Error: console errors or network failures
	if resp.Console != nil && len(resp.Console.Errors) > 0 {
		return "error"
	}
	if resp.Network != nil && len(resp.Network.Failures) > 0 {
		return "error"
	}

	// Warning: console warnings or WebSocket disconnections
	if resp.Console != nil && len(resp.Console.Warnings) > 0 {
		return "warning"
	}
	if resp.WebSocket != nil && len(resp.WebSocket.Disconnections) > 0 {
		return "warning"
	}

	return "clean"
}

func (cm *CheckpointManager) buildSummary(resp DiffResponse) string {
	if resp.Severity == "clean" {
		return "No significant changes."
	}

	var parts []string

	if resp.Console != nil && len(resp.Console.Errors) > 0 {
		count := 0
		for _, e := range resp.Console.Errors {
			count += e.Count
		}
		parts = append(parts, fmt.Sprintf("%d new console error(s)", count))
	}

	if resp.Network != nil && len(resp.Network.Failures) > 0 {
		parts = append(parts, fmt.Sprintf("%d network failure(s)", len(resp.Network.Failures)))
	}

	if resp.Console != nil && len(resp.Console.Warnings) > 0 {
		count := 0
		for _, w := range resp.Console.Warnings {
			count += w.Count
		}
		parts = append(parts, fmt.Sprintf("%d new console warning(s)", count))
	}

	if resp.WebSocket != nil && len(resp.WebSocket.Disconnections) > 0 {
		parts = append(parts, fmt.Sprintf("%d websocket disconnection(s)", len(resp.WebSocket.Disconnections)))
	}

	if len(parts) == 0 {
		return "No significant changes."
	}

	return strings.Join(parts, ", ")
}

// ============================================
// Internal: Known endpoints
// ============================================

func (cm *CheckpointManager) buildKnownEndpoints(existing map[string]endpointState) map[string]endpointState {
	result := make(map[string]endpointState)

	// Copy existing
	for k, v := range existing {
		result[k] = v
	}

	// Update with current network bodies
	cm.capture.mu.RLock()
	for _, body := range cm.capture.networkBodies {
		path := extractURLPath(body.URL)
		result[path] = endpointState{
			Status:   body.Status,
			Duration: body.Duration,
		}
	}
	cm.capture.mu.RUnlock()

	return result
}

// ============================================
// Utility functions
// ============================================

var (
	uuidRegex      = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	largeNumberRe  = regexp.MustCompile(`\b\d{4,}\b`)
	isoTimestampRe = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z?`)
)

// FingerprintMessage normalizes dynamic content in a message for deduplication
func FingerprintMessage(msg string) string {
	// Replace UUIDs
	result := uuidRegex.ReplaceAllString(msg, "{uuid}")
	// Replace ISO timestamps (before numbers, since timestamps contain numbers)
	result = isoTimestampRe.ReplaceAllString(result, "{ts}")
	// Replace large numbers (4+ digits)
	result = largeNumberRe.ReplaceAllString(result, "{n}")
	return result
}

func truncateMessage(msg string) string {
	if len(msg) <= maxMessageLen {
		return msg
	}
	// Truncate at a valid UTF-8 boundary to avoid splitting multi-byte characters
	truncated := msg[:maxMessageLen]
	for len(truncated) > 0 && !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// ============================================
// Push Regression Alert Constants
// ============================================

const (
	maxPendingAlerts = 10

	// Regression thresholds (from spec)
	loadRegressionPct     = 20.0
	fcpRegressionPct      = 20.0
	lcpRegressionPct      = 20.0
	ttfbRegressionPct     = 50.0
	clsRegressionAbs      = 0.1
	transferRegressionPct = 25.0
)

// ============================================
// Push Regression Alert Detection
// ============================================

// DetectAndStoreAlerts checks the given performance snapshot against the given baseline
// and stores any regression alerts for delivery via get_changes_since.
// The baseline should be the state BEFORE the snapshot was incorporated.
func (cm *CheckpointManager) DetectAndStoreAlerts(snapshot PerformanceSnapshot, baseline PerformanceBaseline) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	url := snapshot.URL

	// Only alert if the baseline has more than 1 sample (first snapshot creates baseline, not alert)
	if baseline.SampleCount < 1 {
		return
	}

	// Detect regressions using push-notification thresholds
	metrics := cm.detectPushRegressions(snapshot, baseline)

	if len(metrics) == 0 {
		// No regression detected — check if any pending alert for this URL should be resolved
		cm.resolveAlertsForURL(url)
		return
	}

	// Remove any existing pending alert for this URL (replaced by the new one)
	cm.resolveAlertsForURL(url)

	// Build summary
	summary := cm.buildAlertSummary(url, metrics)

	// Create the alert
	cm.alertCounter++
	cm.alertDelivery++
	alert := PerformanceAlert{
		ID:             cm.alertCounter,
		Type:           "regression",
		URL:            url,
		DetectedAt:     time.Now().Format(time.RFC3339Nano),
		Summary:        summary,
		Metrics:        metrics,
		Recommendation: "Check recently added scripts or stylesheets. Use check_performance for full details.",
		deliveredAt:    0, // not yet delivered
	}

	cm.pendingAlerts = append(cm.pendingAlerts, alert)

	// Cap at maxPendingAlerts, dropping oldest
	if len(cm.pendingAlerts) > maxPendingAlerts {
		keep := len(cm.pendingAlerts) - maxPendingAlerts
		surviving := make([]PerformanceAlert, maxPendingAlerts)
		copy(surviving, cm.pendingAlerts[keep:])
		cm.pendingAlerts = surviving
	}
}

// detectPushRegressions compares snapshot against baseline using the push-notification thresholds.
// Returns only metrics that exceed their thresholds.
func (cm *CheckpointManager) detectPushRegressions(snapshot PerformanceSnapshot, baseline PerformanceBaseline) map[string]AlertMetricDelta {
	metrics := make(map[string]AlertMetricDelta)

	// Load time: >20% regression
	if baseline.Timing.Load > 0 {
		delta := snapshot.Timing.Load - baseline.Timing.Load
		pct := delta / baseline.Timing.Load * 100
		if pct > loadRegressionPct {
			metrics["load"] = AlertMetricDelta{
				Baseline: baseline.Timing.Load,
				Current:  snapshot.Timing.Load,
				DeltaMs:  delta,
				DeltaPct: pct,
			}
		}
	}

	// FCP: >20% regression
	if snapshot.Timing.FirstContentfulPaint != nil && baseline.Timing.FirstContentfulPaint != nil && *baseline.Timing.FirstContentfulPaint > 0 {
		delta := *snapshot.Timing.FirstContentfulPaint - *baseline.Timing.FirstContentfulPaint
		pct := delta / *baseline.Timing.FirstContentfulPaint * 100
		if pct > fcpRegressionPct {
			metrics["fcp"] = AlertMetricDelta{
				Baseline: *baseline.Timing.FirstContentfulPaint,
				Current:  *snapshot.Timing.FirstContentfulPaint,
				DeltaMs:  delta,
				DeltaPct: pct,
			}
		}
	}

	// LCP: >20% regression
	if snapshot.Timing.LargestContentfulPaint != nil && baseline.Timing.LargestContentfulPaint != nil && *baseline.Timing.LargestContentfulPaint > 0 {
		delta := *snapshot.Timing.LargestContentfulPaint - *baseline.Timing.LargestContentfulPaint
		pct := delta / *baseline.Timing.LargestContentfulPaint * 100
		if pct > lcpRegressionPct {
			metrics["lcp"] = AlertMetricDelta{
				Baseline: *baseline.Timing.LargestContentfulPaint,
				Current:  *snapshot.Timing.LargestContentfulPaint,
				DeltaMs:  delta,
				DeltaPct: pct,
			}
		}
	}

	// TTFB: >50% regression (more tolerance for network variance)
	if baseline.Timing.TimeToFirstByte > 0 {
		delta := snapshot.Timing.TimeToFirstByte - baseline.Timing.TimeToFirstByte
		pct := delta / baseline.Timing.TimeToFirstByte * 100
		if pct > ttfbRegressionPct {
			metrics["ttfb"] = AlertMetricDelta{
				Baseline: baseline.Timing.TimeToFirstByte,
				Current:  snapshot.Timing.TimeToFirstByte,
				DeltaMs:  delta,
				DeltaPct: pct,
			}
		}
	}

	// CLS: >0.1 absolute increase
	if snapshot.CLS != nil && baseline.CLS != nil {
		delta := *snapshot.CLS - *baseline.CLS
		if delta > clsRegressionAbs {
			pct := 0.0
			if *baseline.CLS > 0 {
				pct = delta / *baseline.CLS * 100
			}
			metrics["cls"] = AlertMetricDelta{
				Baseline: *baseline.CLS,
				Current:  *snapshot.CLS,
				DeltaMs:  delta, // for CLS this is the absolute delta, not ms
				DeltaPct: pct,
			}
		}
	}

	// Total transfer size: >25% increase
	if baseline.Network.TransferSize > 0 {
		delta := float64(snapshot.Network.TransferSize - baseline.Network.TransferSize)
		pct := delta / float64(baseline.Network.TransferSize) * 100
		if pct > transferRegressionPct {
			metrics["transfer_bytes"] = AlertMetricDelta{
				Baseline: float64(baseline.Network.TransferSize),
				Current:  float64(snapshot.Network.TransferSize),
				DeltaMs:  delta, // for transfer this is the byte delta
				DeltaPct: pct,
			}
		}
	}

	return metrics
}

// resolveAlertsForURL removes any pending alerts for the given URL
func (cm *CheckpointManager) resolveAlertsForURL(url string) {
	// Use new slice to allow GC of resolved alerts (avoids [:0] backing-array pinning)
	filtered := make([]PerformanceAlert, 0, len(cm.pendingAlerts))
	for _, alert := range cm.pendingAlerts {
		if alert.URL != url {
			filtered = append(filtered, alert)
		}
	}
	cm.pendingAlerts = filtered
}

// buildAlertSummary generates a human-readable summary for an alert
func (cm *CheckpointManager) buildAlertSummary(url string, metrics map[string]AlertMetricDelta) string {
	if loadMetric, ok := metrics["load"]; ok {
		return fmt.Sprintf("Load time regressed by %.0fms (%.0fms -> %.0fms) on %s",
			loadMetric.DeltaMs, loadMetric.Baseline, loadMetric.Current, url)
	}
	// Fallback: mention the first metric found
	for name, metric := range metrics {
		return fmt.Sprintf("%s regressed by %.1f%% on %s", name, metric.DeltaPct, url)
	}
	return fmt.Sprintf("Performance regression detected on %s", url)
}

// ============================================
// Push Regression Alert Delivery
// ============================================

// getPendingAlerts returns alerts that should be included in the response
// based on the checkpoint's alertDelivery counter.
func (cm *CheckpointManager) getPendingAlerts(checkpointDelivery int64) []PerformanceAlert {
	var result []PerformanceAlert
	for i := range cm.pendingAlerts {
		// Include alerts that haven't been delivered yet, or were delivered after this checkpoint
		if cm.pendingAlerts[i].deliveredAt == 0 || cm.pendingAlerts[i].deliveredAt > checkpointDelivery {
			result = append(result, cm.pendingAlerts[i])
		}
	}
	return result
}

// markAlertsDelivered marks all pending alerts as delivered at the current delivery counter
func (cm *CheckpointManager) markAlertsDelivered() {
	for i := range cm.pendingAlerts {
		if cm.pendingAlerts[i].deliveredAt == 0 {
			cm.pendingAlerts[i].deliveredAt = cm.alertDelivery
		}
	}
}
