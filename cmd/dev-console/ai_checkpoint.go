package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
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
	From       time.Time      `json:"from"`
	To         time.Time      `json:"to"`
	DurationMs int64          `json:"duration_ms"`
	Severity   string         `json:"severity"`
	Summary    string         `json:"summary"`
	TokenCount int            `json:"token_count"`
	Console    *ConsoleDiff   `json:"console,omitempty"`
	Network    *NetworkDiff   `json:"network,omitempty"`
	WebSocket  *WebSocketDiff `json:"websocket,omitempty"`
	Actions    *ActionsDiff   `json:"actions,omitempty"`
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

	server  *Server
	capture *Capture
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

// CreateCheckpoint creates a named checkpoint at the current buffer positions
func (cm *CheckpointManager) CreateCheckpoint(name string) error {
	if name == "" {
		return fmt.Errorf("checkpoint name cannot be empty")
	}
	if len(name) > maxCheckpointNameLen {
		return fmt.Errorf("checkpoint name exceeds %d characters", maxCheckpointNameLen)
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cp := cm.snapshotNow()
	cp.Name = name

	// If name already exists, update it
	if _, exists := cm.namedCheckpoints[name]; !exists {
		cm.namedOrder = append(cm.namedOrder, name)
	}
	cm.namedCheckpoints[name] = cp

	// Enforce max named checkpoints (evict oldest)
	for len(cm.namedCheckpoints) > maxNamedCheckpoints {
		oldest := cm.namedOrder[0]
		cm.namedOrder = cm.namedOrder[1:]
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

// GetChangesSince computes a compressed diff since the specified checkpoint
func (cm *CheckpointManager) GetChangesSince(params GetChangesSinceParams) DiffResponse {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	isNamedQuery := false

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
	} else if named, ok := cm.namedCheckpoints[params.Checkpoint]; ok {
		// Named checkpoint
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

	// Calculate token count
	jsonBytes, _ := json.Marshal(resp)
	resp.TokenCount = len(jsonBytes) / 4

	// Advance auto-checkpoint (only for auto-mode queries)
	if !isNamedQuery {
		cm.autoCheckpoint = cm.snapshotNow()
		// Update known endpoints from current network state
		cm.autoCheckpoint.KnownEndpoints = cm.buildKnownEndpoints(cp.KnownEndpoints)
	}

	return resp
}

// ============================================
// Internal: Checkpoint resolution
// ============================================

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
	entries := cm.server.entries
	currentTotal := cm.server.logTotalAdded
	cm.server.mu.RUnlock()

	// Calculate how many entries to read
	newCount := int(currentTotal - cp.LogTotal)
	if newCount <= 0 {
		return &ConsoleDiff{}
	}

	// Best-effort: if more new entries than buffer holds, read all available
	available := len(entries)
	toRead := newCount
	if toRead > available {
		toRead = available
	}

	// Get the last toRead entries
	newEntries := entries[available-toRead:]

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
	bodies := cm.capture.networkBodies
	currentTotal := cm.capture.networkTotalAdded
	cm.capture.mu.RUnlock()

	newCount := int(currentTotal - cp.NetworkTotal)
	if newCount <= 0 {
		return &NetworkDiff{}
	}

	available := len(bodies)
	toRead := newCount
	if toRead > available {
		toRead = available
	}

	newBodies := bodies[available-toRead:]

	diff := &NetworkDiff{TotalNew: len(newBodies)}

	// Track endpoints seen in new entries
	for _, body := range newBodies {
		path := ExtractURLPath(body.URL)

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
	events := cm.capture.wsEvents
	currentTotal := cm.capture.wsTotalAdded
	cm.capture.mu.RUnlock()

	newCount := int(currentTotal - cp.WSTotal)
	if newCount <= 0 {
		return &WebSocketDiff{}
	}

	available := len(events)
	toRead := newCount
	if toRead > available {
		toRead = available
	}

	newEvents := events[available-toRead:]

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
	actions := cm.capture.enhancedActions
	currentTotal := cm.capture.actionTotalAdded
	cm.capture.mu.RUnlock()

	newCount := int(currentTotal - cp.ActionTotal)
	if newCount <= 0 {
		return &ActionsDiff{}
	}

	available := len(actions)
	toRead := newCount
	if toRead > available {
		toRead = available
	}

	newActions := actions[available-toRead:]

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
		path := ExtractURLPath(body.URL)
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

// ExtractURLPath extracts the path from a URL, stripping query parameters
func ExtractURLPath(rawURL string) string {
	// Try parsing as full URL
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	path := parsed.Path
	if path == "" {
		return "/"
	}
	return path
}

func truncateMessage(msg string) string {
	if len(msg) > maxMessageLen {
		return msg[:maxMessageLen]
	}
	return msg
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
