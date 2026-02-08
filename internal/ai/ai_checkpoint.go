// ai_checkpoint.go — Named checkpoint/diff system for session state comparison.
// Checkpoints record buffer positions at a point in time. Diffs return only
// new entries since the checkpoint, deduplicated and severity-filtered.
// Design: Checkpoints store position counters, not data copies, making them
// cheap to create. Max 20 named checkpoints with LRU eviction. Auto-checkpoint
// advances on each diff call for "show me what's new" workflows.
package ai

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/server"
	gasTypes "github.com/dev-console/dev-console/internal/types"
)

// Type aliases for types from capture package
type NetworkBody = capture.NetworkBody
type WebSocketEvent = capture.WebSocketEvent
type EnhancedAction = capture.EnhancedAction

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
	PerformanceAlerts []gasTypes.PerformanceAlert `json:"performance_alerts,omitempty"`
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

	server server.LogReader // Use interface instead of concrete *server.Server

	// Push regression alerts
	pendingAlerts []gasTypes.PerformanceAlert
	alertCounter  int64
	alertDelivery int64 // monotonic counter for delivery tracking
	capture       *capture.Capture
}

// ============================================
// Constructor
// ============================================

// NewCheckpointManager creates a new checkpoint manager
func NewCheckpointManager(serverReader server.LogReader, capture *capture.Capture) *CheckpointManager {
	return &CheckpointManager{
		namedCheckpoints: make(map[string]*Checkpoint),
		namedOrder:       make([]string, 0),
		server:           serverReader,
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
	logTotal := cm.server.GetLogTotalAdded()
	netTotal := cm.capture.GetNetworkTotalAdded()
	wsTotal := cm.capture.GetWebSocketTotalAdded()
	actTotal := cm.capture.GetActionTotalAdded()

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
	logTotal := cm.findPositionAtTime(cm.server.GetLogTimestamps(), cm.server.GetLogTotalAdded(), t)
	netTotal := cm.findPositionAtTime(cm.capture.GetNetworkTimestamps(), cm.capture.GetNetworkTotalAdded(), t)
	wsTotal := cm.findPositionAtTime(cm.capture.GetWebSocketTimestamps(), cm.capture.GetWebSocketTotalAdded(), t)
	actTotal := cm.findPositionAtTime(cm.capture.GetActionTimestamps(), cm.capture.GetActionTotalAdded(), t)

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
