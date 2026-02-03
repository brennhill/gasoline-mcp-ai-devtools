// sessions.go — Session Comparison (diff_sessions) MCP tool.
// Stores named snapshots of browser state and compares them to detect
// regressions, improvements, and changes across deployments or code changes.
package session

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/performance"
)

// ============================================
// Constants
// ============================================

const (
	maxSnapshotNameLen     = 50
	maxConsolePerSnapshot  = 50
	maxNetworkPerSnapshot  = 100
	reservedSnapshotName   = "current"
	// Performance regression threshold: >50% increase
	perfRegressionRatio    = 1.5
)

// ============================================
// Types
// ============================================

// CaptureStateReader abstracts reading current server state for snapshot capture.
type CaptureStateReader interface {
	GetConsoleErrors() []SnapshotError
	GetConsoleWarnings() []SnapshotError
	GetNetworkRequests() []SnapshotNetworkRequest
	GetWSConnections() []SnapshotWSConnection
	GetPerformance() *performance.PerformanceSnapshot
	GetCurrentPageURL() string
}

// SnapshotError represents a console error or warning in a snapshot.
type SnapshotError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Count   int    `json:"count"`
}

// SnapshotNetworkRequest represents a network request in a snapshot.
type SnapshotNetworkRequest struct {
	Method       string `json:"method"`
	URL          string `json:"url"`
	Status       int    `json:"status"`
	Duration     int    `json:"duration,omitempty"`
	ResponseSize int    `json:"response_size,omitempty"`
	ContentType  string `json:"content_type,omitempty"`
}

// SnapshotWSConnection represents a WebSocket connection in a snapshot.
type SnapshotWSConnection struct {
	URL         string  `json:"url"`
	State       string  `json:"state"`
	MessageRate float64 `json:"message_rate,omitempty"`
}

// NamedSnapshot is a stored point-in-time browser state.
type NamedSnapshot struct {
	Name                 string                   `json:"name"`
	CapturedAt           time.Time                `json:"captured_at"`
	URLFilter            string                   `json:"url,omitempty"`
	PageURL              string                   `json:"page_url"`
	ConsoleErrors        []SnapshotError          `json:"console_errors"`
	ConsoleWarnings      []SnapshotError          `json:"console_warnings"`
	NetworkRequests      []SnapshotNetworkRequest `json:"network_requests"`
	WebSocketConnections []SnapshotWSConnection   `json:"websocket_connections"`
	Performance          *performance.PerformanceSnapshot     `json:"performance,omitempty"`
}

// SnapshotListEntry is a summary of a snapshot for the list response.
type SnapshotListEntry struct {
	Name       string    `json:"name"`
	CapturedAt time.Time `json:"captured_at"`
	PageURL    string    `json:"page_url"`
	ErrorCount int       `json:"error_count"`
}

// SessionDiffResult is the full comparison result between two snapshots.
type SessionDiffResult struct {
	A           string             `json:"a"`
	B           string             `json:"b"`
	Errors      ErrorDiff          `json:"errors"`
	Network     SessionNetworkDiff `json:"network"`
	Performance PerformanceDiff    `json:"performance"`
	Summary     DiffSummary        `json:"summary"`
}

// ErrorDiff holds the error comparison between two snapshots.
type ErrorDiff struct {
	New       []SnapshotError `json:"new"`
	Resolved  []SnapshotError `json:"resolved"`
	Unchanged []SnapshotError `json:"unchanged"`
}

// SessionNetworkDiff holds the network comparison between two snapshots.
type SessionNetworkDiff struct {
	NewErrors        []SnapshotNetworkRequest `json:"new_errors"`
	StatusChanges    []SessionNetworkChange   `json:"status_changes"`
	NewEndpoints     []SnapshotNetworkRequest `json:"new_endpoints"`
	MissingEndpoints []SnapshotNetworkRequest `json:"missing_endpoints"`
}

// SessionNetworkChange represents a status code change for the same endpoint.
type SessionNetworkChange struct {
	Method         string `json:"method"`
	URL            string `json:"url"`
	BeforeStatus   int    `json:"before"`
	AfterStatus    int    `json:"after"`
	DurationChange string `json:"duration_change,omitempty"`
}

// PerformanceDiff holds performance metric comparisons.
type PerformanceDiff struct {
	LoadTime     *MetricChange `json:"load_time,omitempty"`
	RequestCount *MetricChange `json:"request_count,omitempty"`
	TransferSize *MetricChange `json:"transfer_size,omitempty"`
}

// MetricChange holds before/after values for a numeric metric.
type MetricChange struct {
	Before     float64 `json:"before"`
	After      float64 `json:"after"`
	Change     string  `json:"change"`
	Regression bool    `json:"regression"`
}

// DiffSummary holds aggregate comparison stats and verdict.
type DiffSummary struct {
	Verdict                string `json:"verdict"`
	NewErrors              int    `json:"new_errors"`
	ResolvedErrors         int    `json:"resolved_errors"`
	PerformanceRegressions int    `json:"performance_regressions"`
	NewNetworkErrors       int    `json:"new_network_errors"`
}

// SessionManager manages named session snapshots.
type SessionManager struct {
	mu      sync.RWMutex
	snaps   map[string]*NamedSnapshot
	order   []string
	maxSize int
	reader  CaptureStateReader
}

// ============================================
// Constructor
// ============================================

// NewSessionManager creates a new SessionManager with the given max snapshot count.
func NewSessionManager(maxSnapshots int, reader CaptureStateReader) *SessionManager {
	if maxSnapshots <= 0 {
		maxSnapshots = 10
	}
	return &SessionManager{
		snaps:   make(map[string]*NamedSnapshot),
		order:   make([]string, 0),
		maxSize: maxSnapshots,
		reader:  reader,
	}
}

// ============================================
// Capture
// ============================================

// Capture stores the current state as a named snapshot.
func (sm *SessionManager) Capture(name, urlFilter string) (*NamedSnapshot, error) {
	if err := sm.validateName(name); err != nil {
		return nil, err
	}

	snapshot := sm.captureCurrentState(name, urlFilter)

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// If name already exists, overwrite (remove from order, re-add at end)
	if _, exists := sm.snaps[name]; exists {
		sm.removeFromOrder(name)
	} else {
		// Evict oldest if at capacity
		for len(sm.order) >= sm.maxSize {
			oldest := sm.order[0]
			delete(sm.snaps, oldest)
			newOrder := make([]string, len(sm.order)-1)
			copy(newOrder, sm.order[1:])
			sm.order = newOrder
		}
	}

	sm.snaps[name] = snapshot
	sm.order = append(sm.order, name)

	return snapshot, nil
}

// captureCurrentState reads the current state from the reader and builds a snapshot.
func (sm *SessionManager) captureCurrentState(name, urlFilter string) *NamedSnapshot {
	errors := sm.reader.GetConsoleErrors()
	warnings := sm.reader.GetConsoleWarnings()
	network := sm.reader.GetNetworkRequests()
	ws := sm.reader.GetWSConnections()
	perf := sm.reader.GetPerformance()
	pageURL := sm.reader.GetCurrentPageURL()

	// Apply URL filter to network requests
	if urlFilter != "" {
		filtered := make([]SnapshotNetworkRequest, 0, len(network))
		for _, req := range network {
			if strings.Contains(req.URL, urlFilter) {
				filtered = append(filtered, req)
			}
		}
		network = filtered
	}

	// Apply limits
	if len(errors) > maxConsolePerSnapshot {
		errors = errors[:maxConsolePerSnapshot]
	}
	if len(warnings) > maxConsolePerSnapshot {
		warnings = warnings[:maxConsolePerSnapshot]
	}
	if len(network) > maxNetworkPerSnapshot {
		network = network[:maxNetworkPerSnapshot]
	}

	// Deep copy performance snapshot if present
	var perfCopy *performance.PerformanceSnapshot
	if perf != nil {
		p := *perf
		perfCopy = &p
	}

	return &NamedSnapshot{
		Name:                 name,
		CapturedAt:           time.Now(),
		URLFilter:            urlFilter,
		PageURL:              pageURL,
		ConsoleErrors:        errors,
		ConsoleWarnings:      warnings,
		NetworkRequests:      network,
		WebSocketConnections: ws,
		Performance:          perfCopy,
	}
}

// ============================================
// Compare
// ============================================

// Compare diffs two snapshots. Use "current" as b to compare against live state.
func (sm *SessionManager) Compare(a, b string) (*SessionDiffResult, error) {
	sm.mu.RLock()
	snapA, existsA := sm.snaps[a]
	sm.mu.RUnlock()

	if !existsA {
		return nil, fmt.Errorf("snapshot %q not found", a)
	}

	var snapB *NamedSnapshot
	if b == reservedSnapshotName {
		// Compare against current live state
		snapB = sm.captureCurrentState("current", snapA.URLFilter)
	} else {
		sm.mu.RLock()
		found, exists := sm.snaps[b]
		sm.mu.RUnlock()
		if !exists {
			return nil, fmt.Errorf("snapshot %q not found", b)
		}
		snapB = found
	}

	result := &SessionDiffResult{
		A: a,
		B: b,
	}

	// Compute error diff
	result.Errors = sm.diffErrors(snapA, snapB)

	// Compute network diff
	result.Network = sm.diffNetwork(snapA, snapB)

	// Compute performance diff
	result.Performance = sm.diffPerformance(snapA, snapB)

	// Compute summary and verdict
	result.Summary = sm.computeSummary(result)

	return result, nil
}

// diffErrors computes the set difference of errors between two snapshots.
func (sm *SessionManager) diffErrors(a, b *NamedSnapshot) ErrorDiff {
	diff := ErrorDiff{
		New:       make([]SnapshotError, 0),
		Resolved:  make([]SnapshotError, 0),
		Unchanged: make([]SnapshotError, 0),
	}

	aMessages := make(map[string]SnapshotError)
	for _, e := range a.ConsoleErrors {
		aMessages[e.Message] = e
	}

	bMessages := make(map[string]SnapshotError)
	for _, e := range b.ConsoleErrors {
		bMessages[e.Message] = e
	}

	// New = in B but not in A
	for msg, e := range bMessages {
		if _, found := aMessages[msg]; !found {
			diff.New = append(diff.New, e)
		}
	}

	// Resolved = in A but not in B
	for msg, e := range aMessages {
		if _, found := bMessages[msg]; !found {
			diff.Resolved = append(diff.Resolved, e)
		} else {
			diff.Unchanged = append(diff.Unchanged, e)
		}
	}

	return diff
}

// diffNetwork compares network requests between two snapshots.
// Requests are matched by (method, URL path) — query params are ignored.
func (sm *SessionManager) diffNetwork(a, b *NamedSnapshot) SessionNetworkDiff {
	diff := SessionNetworkDiff{
		NewErrors:        make([]SnapshotNetworkRequest, 0),
		StatusChanges:    make([]SessionNetworkChange, 0),
		NewEndpoints:     make([]SnapshotNetworkRequest, 0),
		MissingEndpoints: make([]SnapshotNetworkRequest, 0),
	}

	type endpointKey struct {
		Method string
		Path   string
	}

	// Build maps by (method, path)
	aEndpoints := make(map[endpointKey]SnapshotNetworkRequest)
	for _, req := range a.NetworkRequests {
		key := endpointKey{Method: req.Method, Path: capture.ExtractURLPath(req.URL)}
		aEndpoints[key] = req
	}

	bEndpoints := make(map[endpointKey]SnapshotNetworkRequest)
	for _, req := range b.NetworkRequests {
		key := endpointKey{Method: req.Method, Path: capture.ExtractURLPath(req.URL)}
		bEndpoints[key] = req
	}

	// New endpoints = in B but not A
	for key, req := range bEndpoints {
		if _, found := aEndpoints[key]; !found {
			diff.NewEndpoints = append(diff.NewEndpoints, req)
			// If the new endpoint is an error (4xx/5xx), also add to new_errors
			if req.Status >= 400 {
				diff.NewErrors = append(diff.NewErrors, req)
			}
		}
	}

	// Missing endpoints = in A but not B
	for key, req := range aEndpoints {
		if _, found := bEndpoints[key]; !found {
			diff.MissingEndpoints = append(diff.MissingEndpoints, req)
		}
	}

	// Status changes = same endpoint, different status
	for key, aReq := range aEndpoints {
		if bReq, found := bEndpoints[key]; found {
			if aReq.Status != bReq.Status {
				change := SessionNetworkChange{
					Method:       key.Method,
					URL:          aReq.URL,
					BeforeStatus: aReq.Status,
					AfterStatus:  bReq.Status,
				}
				// Compute duration change if both have duration
				if aReq.Duration > 0 && bReq.Duration > 0 {
					delta := bReq.Duration - aReq.Duration
					if delta >= 0 {
						change.DurationChange = fmt.Sprintf("+%dms", delta)
					} else {
						change.DurationChange = fmt.Sprintf("%dms", delta)
					}
				}
				diff.StatusChanges = append(diff.StatusChanges, change)
				// A status change to 4xx/5xx is also a new error
				if bReq.Status >= 400 && aReq.Status < 400 {
					diff.NewErrors = append(diff.NewErrors, bReq)
				}
			}
		}
	}

	return diff
}

// diffPerformance compares performance metrics between two snapshots.
func (sm *SessionManager) diffPerformance(a, b *NamedSnapshot) PerformanceDiff {
	diff := PerformanceDiff{}

	if a.Performance == nil || b.Performance == nil {
		return diff
	}

	// Load time comparison
	if a.Performance.Timing.Load > 0 || b.Performance.Timing.Load > 0 {
		diff.LoadTime = computeMetricChange(
			a.Performance.Timing.Load,
			b.Performance.Timing.Load,
		)
	}

	// Request count comparison
	if a.Performance.Network.RequestCount > 0 || b.Performance.Network.RequestCount > 0 {
		diff.RequestCount = computeMetricChange(
			float64(a.Performance.Network.RequestCount),
			float64(b.Performance.Network.RequestCount),
		)
	}

	// Transfer size comparison
	if a.Performance.Network.TransferSize > 0 || b.Performance.Network.TransferSize > 0 {
		diff.TransferSize = computeMetricChange(
			float64(a.Performance.Network.TransferSize),
			float64(b.Performance.Network.TransferSize),
		)
	}

	return diff
}

// computeMetricChange creates a MetricChange comparing two values.
func computeMetricChange(before, after float64) *MetricChange {
	mc := &MetricChange{
		Before: before,
		After:  after,
	}

	if before == 0 {
		if after > 0 {
			mc.Change = "+inf"
			mc.Regression = true
		} else {
			mc.Change = "0%"
		}
		return mc
	}

	pctChange := ((after - before) / before) * 100
	if pctChange >= 0 {
		mc.Change = fmt.Sprintf("+%.0f%%", pctChange)
	} else {
		mc.Change = fmt.Sprintf("%.0f%%", pctChange)
	}

	// Regression = after > before * threshold
	mc.Regression = after > before*perfRegressionRatio

	return mc
}

// computeSummary derives the verdict and aggregate counts from the diff.
func (sm *SessionManager) computeSummary(result *SessionDiffResult) DiffSummary {
	summary := DiffSummary{
		NewErrors:      len(result.Errors.New),
		ResolvedErrors: len(result.Errors.Resolved),
		NewNetworkErrors: len(result.Network.NewErrors),
	}

	// Count performance regressions
	if result.Performance.LoadTime != nil && result.Performance.LoadTime.Regression {
		summary.PerformanceRegressions++
	}
	if result.Performance.RequestCount != nil && result.Performance.RequestCount.Regression {
		summary.PerformanceRegressions++
	}
	if result.Performance.TransferSize != nil && result.Performance.TransferSize.Regression {
		summary.PerformanceRegressions++
	}

	// Verdict logic:
	// "improved" if resolved > 0 AND new == 0 AND no regressions
	// "regressed" if new > 0 OR performance_regressions > 0 OR new_network_errors > 0
	// "unchanged" if no differences
	// "mixed" if both resolved and new

	hasRegressions := summary.NewErrors > 0 || summary.PerformanceRegressions > 0 || summary.NewNetworkErrors > 0
	hasImprovements := summary.ResolvedErrors > 0

	// Also check for status changes where a previously-OK endpoint now errors
	for _, sc := range result.Network.StatusChanges {
		if sc.AfterStatus >= 400 && sc.BeforeStatus < 400 {
			hasRegressions = true
			break
		}
	}

	switch {
	case hasRegressions && hasImprovements:
		summary.Verdict = "mixed"
	case hasRegressions:
		summary.Verdict = "regressed"
	case hasImprovements:
		summary.Verdict = "improved"
	default:
		summary.Verdict = "unchanged"
	}

	return summary
}

// ============================================
// List
// ============================================

// List returns all stored snapshots in insertion order.
func (sm *SessionManager) List() []SnapshotListEntry {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	entries := make([]SnapshotListEntry, 0, len(sm.order))
	for _, name := range sm.order {
		snap := sm.snaps[name]
		entries = append(entries, SnapshotListEntry{
			Name:       snap.Name,
			CapturedAt: snap.CapturedAt,
			PageURL:    snap.PageURL,
			ErrorCount: len(snap.ConsoleErrors),
		})
	}
	return entries
}

// ============================================
// Delete
// ============================================

// Delete removes a named snapshot.
func (sm *SessionManager) Delete(name string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.snaps[name]; !exists {
		return fmt.Errorf("snapshot %q not found", name)
	}

	delete(sm.snaps, name)
	sm.removeFromOrder(name)
	return nil
}

// ============================================
// Tool Handler
// ============================================

// diffSessionsParams defines the MCP tool input schema.
type diffSessionsParams struct {
	Action    string `json:"action"`
	Name      string `json:"name,omitempty"`
	CompareA  string `json:"compare_a,omitempty"`
	CompareB  string `json:"compare_b,omitempty"`
	URLFilter string `json:"url,omitempty"`
}

// HandleTool dispatches the diff_sessions MCP tool call.
func (sm *SessionManager) HandleTool(params json.RawMessage) (any, error) {
	var p diffSessionsParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	switch p.Action {
	case "capture":
		if p.Name == "" {
			return nil, fmt.Errorf("'name' is required for capture action")
		}
		snap, err := sm.Capture(p.Name, p.URLFilter)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"action": "captured",
			"name":   snap.Name,
			"snapshot": map[string]any{
				"captured_at":      snap.CapturedAt,
				"console_errors":   len(snap.ConsoleErrors),
				"console_warnings": len(snap.ConsoleWarnings),
				"network_requests": len(snap.NetworkRequests),
				"page_url":         snap.PageURL,
			},
		}, nil

	case "compare":
		if p.CompareA == "" || p.CompareB == "" {
			return nil, fmt.Errorf("'compare_a' and 'compare_b' are required for compare action")
		}
		diff, err := sm.Compare(p.CompareA, p.CompareB)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"action":  "compared",
			"a":       diff.A,
			"b":       diff.B,
			"diff":    diff,
			"summary": diff.Summary,
		}, nil

	case "list":
		entries := sm.List()
		return map[string]any{
			"action":    "listed",
			"snapshots": entries,
		}, nil

	case "delete":
		if p.Name == "" {
			return nil, fmt.Errorf("'name' is required for delete action")
		}
		if err := sm.Delete(p.Name); err != nil {
			return nil, err
		}
		return map[string]any{
			"action": "deleted",
			"name":   p.Name,
		}, nil

	default:
		if p.Action == "" {
			return nil, fmt.Errorf("'action' is required (capture, compare, list, delete)")
		}
		return nil, fmt.Errorf("unknown action %q (valid: capture, compare, list, delete)", p.Action)
	}
}

// ============================================
// Helpers
// ============================================

// validateName checks snapshot name constraints.
func (sm *SessionManager) validateName(name string) error {
	if name == "" {
		return fmt.Errorf("snapshot name cannot be empty")
	}
	if name == reservedSnapshotName {
		return fmt.Errorf("snapshot name %q is reserved", reservedSnapshotName)
	}
	if len(name) > maxSnapshotNameLen {
		return fmt.Errorf("snapshot name exceeds %d characters", maxSnapshotNameLen)
	}
	return nil
}

// removeFromOrder removes a name from the insertion order slice.
func (sm *SessionManager) removeFromOrder(name string) {
	for i, n := range sm.order {
		if n == name {
			newOrder := make([]string, len(sm.order)-1)
			copy(newOrder, sm.order[:i])
			copy(newOrder[i:], sm.order[i+1:])
			sm.order = newOrder
			return
		}
	}
}

// extractPath returns just the path component of a URL, stripping query params.

// Ensure SessionManager is not confused with unused imports
