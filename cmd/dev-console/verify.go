// verify.go — Verification Loop (verify_fix) MCP tool.
// Provides before/after session comparison for fix verification.
// AI captures state before a fix, applies fix, captures after, compares to verify.
// Session lifecycle: start -> watch -> compare (or cancel at any point).
package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================
// Constants
// ============================================

const (
	maxVerificationSessions   = 3
	defaultVerificationTTL    = 30 * time.Minute
	maxBaselineErrors         = 50
	maxBaselineNetworkEntries = 50
)

// ============================================
// Normalization Regex Patterns (verify-specific)
// ============================================

var (
	// File path with line number: filename.ext:123
	// More specific pattern than clustering.go uses
	verifyFileLineRegex = regexp.MustCompile(`[\w./\\-]+\.[a-zA-Z]{1,4}:\d+`)
)

// ============================================
// Types
// ============================================

// VerificationSession tracks a before/after comparison
type VerificationSession struct {
	ID        string    `json:"session_id"`
	Label     string    `json:"label"`
	Status    string    `json:"status"` // "baseline_captured", "watching", "compared", "cancelled"
	URLFilter string    `json:"url,omitempty"`
	CreatedAt time.Time `json:"created_at"`

	// Baseline snapshot (captured at "start")
	Baseline *SessionSnapshot `json:"baseline"`

	// Watch state
	WatchStartedAt *time.Time `json:"watch_started_at,omitempty"`

	// After snapshot (captured at "compare")
	After *SessionSnapshot `json:"after,omitempty"`
}

// SessionSnapshot is a point-in-time capture of browser state for verification
type SessionSnapshot struct {
	CapturedAt         time.Time            `json:"captured_at"`
	ConsoleErrors      []VerifyError        `json:"console_errors"`
	NetworkErrors      []VerifyNetworkEntry `json:"network_errors"`      // Only 4xx/5xx responses
	AllNetworkRequests []VerifyNetworkEntry `json:"all_network,omitempty"` // All requests (for comparison)
	PageURL            string               `json:"page_url,omitempty"`
	Performance        *PerformanceSnapshot `json:"performance,omitempty"`
}

// VerifyError represents a console error in a verification snapshot
type VerifyError struct {
	Message    string `json:"message"`
	Normalized string `json:"normalized"` // For matching
	Count      int    `json:"count"`
	Source     string `json:"source,omitempty"`
}

// VerifyNetworkEntry represents a network request in a verification snapshot
type VerifyNetworkEntry struct {
	Method   string `json:"method"`
	URL      string `json:"url"`
	Path     string `json:"path"` // URL path only (for matching)
	Status   int    `json:"status"`
	Duration int    `json:"duration,omitempty"`
}

// ============================================
// Response Types
// ============================================

// StartResult is the response from the start action
type StartResult struct {
	SessionID string             `json:"session_id"`
	Label     string             `json:"label,omitempty"`
	Status    string             `json:"status"`
	Baseline  BaselineSummary    `json:"baseline"`
}

// BaselineSummary summarizes the captured baseline state
type BaselineSummary struct {
	CapturedAt    time.Time        `json:"captured_at"`
	ConsoleErrors int              `json:"console_errors"`
	NetworkErrors int              `json:"network_errors"`
	ErrorDetails  []ErrorDetail    `json:"error_details"`
}

// ErrorDetail describes a single error in the baseline
type ErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Count   int    `json:"count,omitempty"`
	Method  string `json:"method,omitempty"`
	URL     string `json:"url,omitempty"`
	Status  int    `json:"status,omitempty"`
}

// WatchResult is the response from the watch action
type WatchResult struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// CompareResult is the response from the compare action
type CompareResult struct {
	SessionID string             `json:"session_id"`
	Status    string             `json:"status"`
	Label     string             `json:"label,omitempty"`
	Result    VerificationResult `json:"result"`
}

// VerificationResult is the diff output
type VerificationResult struct {
	Verdict         string           `json:"verdict"`
	Before          IssueSummary     `json:"before"`
	After           IssueSummary     `json:"after"`
	Changes         []VerifyChange   `json:"changes"`
	NewIssues       []VerifyChange   `json:"new_issues"`
	PerformanceDiff *VerifyPerfDiff  `json:"performance_diff,omitempty"`
}

// IssueSummary provides aggregate error counts
type IssueSummary struct {
	ConsoleErrors int `json:"console_errors"`
	NetworkErrors int `json:"network_errors"`
	TotalIssues   int `json:"total_issues"`
}

// VerifyChange describes a single change between before and after
type VerifyChange struct {
	Type     string `json:"type"` // "resolved", "new", "changed", "unchanged"
	Category string `json:"category"` // "console", "network"
	Before   string `json:"before"`
	After    string `json:"after"`
}

// VerifyPerfDiff holds performance change summary
type VerifyPerfDiff struct {
	LoadTimeBefore string `json:"load_time_before,omitempty"`
	LoadTimeAfter  string `json:"load_time_after,omitempty"`
	Change         string `json:"change,omitempty"`
}

// CancelResult is the response from the cancel action
type CancelResult struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
}

// StatusResult is the response from the status action
type StatusResult struct {
	SessionID string    `json:"session_id"`
	Label     string    `json:"label,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// ============================================
// VerificationManager
// ============================================

// VerificationManager manages verification sessions
type VerificationManager struct {
	mu       sync.RWMutex
	sessions map[string]*VerificationSession
	order    []string // Track insertion order
	reader   CaptureStateReader
	ttl      time.Duration
	idSeq    int
}

// NewVerificationManager creates a new VerificationManager
func NewVerificationManager(reader CaptureStateReader) *VerificationManager {
	return &VerificationManager{
		sessions: make(map[string]*VerificationSession),
		order:    make([]string, 0),
		reader:   reader,
		ttl:      defaultVerificationTTL,
	}
}

// NewVerificationManagerWithTTL creates a new VerificationManager with custom TTL
func NewVerificationManagerWithTTL(reader CaptureStateReader, ttl time.Duration) *VerificationManager {
	return &VerificationManager{
		sessions: make(map[string]*VerificationSession),
		order:    make([]string, 0),
		reader:   reader,
		ttl:      ttl,
	}
}

// ============================================
// Start Action
// ============================================

// Start captures the baseline (broken) state
func (vm *VerificationManager) Start(label, urlFilter string) (*StartResult, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Clean up expired sessions first
	vm.cleanupExpiredLocked()

	// Check session limit
	if len(vm.sessions) >= maxVerificationSessions {
		return nil, fmt.Errorf("maximum concurrent verification sessions (%d) reached", maxVerificationSessions)
	}

	// Generate session ID
	vm.idSeq++
	sessionID := fmt.Sprintf("verify-%d-%d", time.Now().Unix(), vm.idSeq)

	// Capture current state as baseline
	baseline := vm.captureSnapshot(urlFilter)

	session := &VerificationSession{
		ID:        sessionID,
		Label:     label,
		Status:    "baseline_captured",
		URLFilter: urlFilter,
		CreatedAt: time.Now(),
		Baseline:  baseline,
	}

	vm.sessions[sessionID] = session
	vm.order = append(vm.order, sessionID)

	// Build response
	result := &StartResult{
		SessionID: sessionID,
		Label:     label,
		Status:    "baseline_captured",
		Baseline:  vm.buildBaselineSummary(baseline),
	}

	return result, nil
}

// ============================================
// Watch Action
// ============================================

// Watch begins monitoring for new activity
func (vm *VerificationManager) Watch(sessionID string) (*WatchResult, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	session, exists := vm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	// Check if expired
	if time.Since(session.CreatedAt) > vm.ttl {
		delete(vm.sessions, sessionID)
		vm.removeFromOrder(sessionID)
		return nil, fmt.Errorf("session %q has expired", sessionID)
	}

	// Check status
	if session.Status == "cancelled" {
		return nil, fmt.Errorf("session %q has been cancelled", sessionID)
	}

	// Set watch state (idempotent)
	now := time.Now()
	session.WatchStartedAt = &now
	session.Status = "watching"

	return &WatchResult{
		SessionID: sessionID,
		Status:    "watching",
		Message:   "Monitoring started. Ask the user to reproduce the scenario.",
	}, nil
}

// ============================================
// Compare Action
// ============================================

// Compare diffs baseline vs current state
func (vm *VerificationManager) Compare(sessionID string) (*CompareResult, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	session, exists := vm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	// Check if expired
	if time.Since(session.CreatedAt) > vm.ttl {
		delete(vm.sessions, sessionID)
		vm.removeFromOrder(sessionID)
		return nil, fmt.Errorf("session %q has expired", sessionID)
	}

	// Must have called watch first
	if session.WatchStartedAt == nil {
		return nil, fmt.Errorf("session %q: must call 'watch' before 'compare'", sessionID)
	}

	// Capture current state as "after"
	after := vm.captureSnapshot(session.URLFilter)
	session.After = after
	session.Status = "compared"

	// Compute diff
	result := vm.computeVerification(session.Baseline, after)

	return &CompareResult{
		SessionID: sessionID,
		Status:    "compared",
		Label:     session.Label,
		Result:    result,
	}, nil
}

// ============================================
// Cancel Action
// ============================================

// Cancel discards a verification session
func (vm *VerificationManager) Cancel(sessionID string) (*CancelResult, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	session, exists := vm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	session.Status = "cancelled"
	delete(vm.sessions, sessionID)
	vm.removeFromOrder(sessionID)

	return &CancelResult{
		SessionID: sessionID,
		Status:    "cancelled",
	}, nil
}

// ============================================
// Status Action
// ============================================

// Status returns the current state of a session
func (vm *VerificationManager) Status(sessionID string) (*StatusResult, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	session, exists := vm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	return &StatusResult{
		SessionID: sessionID,
		Label:     session.Label,
		Status:    session.Status,
		CreatedAt: session.CreatedAt,
	}, nil
}

// ============================================
// Snapshot Capture
// ============================================

// captureSnapshot captures current state from the reader
func (vm *VerificationManager) captureSnapshot(urlFilter string) *SessionSnapshot {
	errors := vm.reader.GetConsoleErrors()
	network := vm.reader.GetNetworkRequests()
	perf := vm.reader.GetPerformance()
	pageURL := vm.reader.GetCurrentPageURL()

	// Convert console errors
	verifyErrors := make([]VerifyError, 0, len(errors))
	for _, e := range errors {
		verifyErrors = append(verifyErrors, VerifyError{
			Message:    e.Message,
			Normalized: normalizeVerifyErrorMessage(e.Message),
			Count:      e.Count,
		})
	}

	// Convert and filter network requests
	networkErrors := make([]VerifyNetworkEntry, 0)
	allNetwork := make([]VerifyNetworkEntry, 0, len(network))
	for _, req := range network {
		// Apply URL filter
		if urlFilter != "" && !strings.Contains(req.URL, urlFilter) {
			continue
		}
		entry := VerifyNetworkEntry{
			Method:   req.Method,
			URL:      req.URL,
			Path:     extractURLPath(req.URL),
			Status:   req.Status,
			Duration: req.Duration,
		}
		allNetwork = append(allNetwork, entry)
		// Track errors separately (4xx/5xx)
		if req.Status >= 400 {
			networkErrors = append(networkErrors, entry)
		}
	}

	// Apply limits
	if len(verifyErrors) > maxBaselineErrors {
		verifyErrors = verifyErrors[:maxBaselineErrors]
	}
	if len(networkErrors) > maxBaselineNetworkEntries {
		networkErrors = networkErrors[:maxBaselineNetworkEntries]
	}
	if len(allNetwork) > maxBaselineNetworkEntries {
		allNetwork = allNetwork[:maxBaselineNetworkEntries]
	}

	// Deep copy performance
	var perfCopy *PerformanceSnapshot
	if perf != nil {
		p := *perf
		perfCopy = &p
	}

	return &SessionSnapshot{
		CapturedAt:         time.Now(),
		ConsoleErrors:      verifyErrors,
		NetworkErrors:      networkErrors,
		AllNetworkRequests: allNetwork,
		PageURL:            pageURL,
		Performance:        perfCopy,
	}
}

// buildBaselineSummary creates a summary of the baseline snapshot
func (vm *VerificationManager) buildBaselineSummary(baseline *SessionSnapshot) BaselineSummary {
	// Count total console errors
	consoleErrorCount := 0
	for _, e := range baseline.ConsoleErrors {
		consoleErrorCount += e.Count
	}

	// Build error details
	details := make([]ErrorDetail, 0, len(baseline.ConsoleErrors)+len(baseline.NetworkErrors))

	for _, e := range baseline.ConsoleErrors {
		details = append(details, ErrorDetail{
			Type:    "console",
			Message: e.Message,
			Count:   e.Count,
		})
	}

	for _, n := range baseline.NetworkErrors {
		details = append(details, ErrorDetail{
			Type:   "network",
			Method: n.Method,
			URL:    n.URL,
			Status: n.Status,
		})
	}

	return BaselineSummary{
		CapturedAt:    baseline.CapturedAt,
		ConsoleErrors: consoleErrorCount,
		NetworkErrors: len(baseline.NetworkErrors),
		ErrorDetails:  details,
	}
}

// ============================================
// Verification Computation
// ============================================

// computeVerification compares baseline and after snapshots
func (vm *VerificationManager) computeVerification(before, after *SessionSnapshot) VerificationResult {
	result := VerificationResult{
		Changes:   make([]VerifyChange, 0),
		NewIssues: make([]VerifyChange, 0),
	}

	// Count totals
	beforeConsoleCount := 0
	for _, e := range before.ConsoleErrors {
		beforeConsoleCount += e.Count
	}
	afterConsoleCount := 0
	for _, e := range after.ConsoleErrors {
		afterConsoleCount += e.Count
	}

	result.Before = IssueSummary{
		ConsoleErrors: beforeConsoleCount,
		NetworkErrors: len(before.NetworkErrors),
		TotalIssues:   beforeConsoleCount + len(before.NetworkErrors),
	}
	result.After = IssueSummary{
		ConsoleErrors: afterConsoleCount,
		NetworkErrors: len(after.NetworkErrors),
		TotalIssues:   afterConsoleCount + len(after.NetworkErrors),
	}

	// Compare console errors by normalized message
	beforeMsgs := make(map[string]VerifyError)
	for _, e := range before.ConsoleErrors {
		beforeMsgs[e.Normalized] = e
	}

	afterMsgs := make(map[string]VerifyError)
	for _, e := range after.ConsoleErrors {
		afterMsgs[e.Normalized] = e
	}

	// Resolved = in before but not in after
	for norm, e := range beforeMsgs {
		if _, found := afterMsgs[norm]; !found {
			suffix := ""
			if e.Count > 1 {
				suffix = fmt.Sprintf(" (x%d)", e.Count)
			}
			result.Changes = append(result.Changes, VerifyChange{
				Type:     "resolved",
				Category: "console",
				Before:   e.Message + suffix,
				After:    "(not seen)",
			})
		}
	}

	// New = in after but not in before
	for norm, e := range afterMsgs {
		if _, found := beforeMsgs[norm]; !found {
			result.NewIssues = append(result.NewIssues, VerifyChange{
				Type:     "new",
				Category: "console",
				Before:   "(not seen)",
				After:    e.Message,
			})
		}
	}

	// Compare network errors by method+path
	// Build map of all "after" requests (any status) for checking error resolution
	afterAllNetwork := make(map[string]VerifyNetworkEntry)
	for _, n := range after.AllNetworkRequests {
		key := n.Method + " " + n.Path
		afterAllNetwork[key] = n
	}

	// Build error-only maps
	beforeNetwork := make(map[string]VerifyNetworkEntry)
	for _, n := range before.NetworkErrors {
		key := n.Method + " " + n.Path
		beforeNetwork[key] = n
	}

	afterNetwork := make(map[string]VerifyNetworkEntry)
	for _, n := range after.NetworkErrors {
		key := n.Method + " " + n.Path
		afterNetwork[key] = n
	}

	// Check for resolved network errors
	for key, n := range beforeNetwork {
		if afterN, found := afterNetwork[key]; found {
			// Still an error - check if different
			if afterN.Status != n.Status {
				// Status changed but both are errors
				result.Changes = append(result.Changes, VerifyChange{
					Type:     "changed",
					Category: "network",
					Before:   fmt.Sprintf("%s %s -> %d", n.Method, n.URL, n.Status),
					After:    fmt.Sprintf("%s %s -> %d", afterN.Method, afterN.URL, afterN.Status),
				})
			}
		} else {
			// Error no longer in error list - check if it succeeded or just wasn't called
			if allN, found := afterAllNetwork[key]; found {
				// Endpoint was called - check if it succeeded
				if allN.Status >= 200 && allN.Status < 400 {
					result.Changes = append(result.Changes, VerifyChange{
						Type:     "resolved",
						Category: "network",
						Before:   fmt.Sprintf("%s %s -> %d", n.Method, n.URL, n.Status),
						After:    fmt.Sprintf("%s %s -> %d", allN.Method, allN.URL, allN.Status),
					})
				} else {
					// Still an error but different status
					result.Changes = append(result.Changes, VerifyChange{
						Type:     "changed",
						Category: "network",
						Before:   fmt.Sprintf("%s %s -> %d", n.Method, n.URL, n.Status),
						After:    fmt.Sprintf("%s %s -> %d", allN.Method, allN.URL, allN.Status),
					})
				}
			} else {
				// Endpoint not called - mark as resolved (can't compare)
				result.Changes = append(result.Changes, VerifyChange{
					Type:     "resolved",
					Category: "network",
					Before:   fmt.Sprintf("%s %s -> %d", n.Method, n.URL, n.Status),
					After:    "(not seen)",
				})
			}
		}
	}

	// New network errors
	for key, n := range afterNetwork {
		if _, found := beforeNetwork[key]; !found {
			result.NewIssues = append(result.NewIssues, VerifyChange{
				Type:     "new",
				Category: "network",
				Before:   "(not seen)",
				After:    fmt.Sprintf("%s %s -> %d", n.Method, n.URL, n.Status),
			})
		}
	}

	// Performance diff
	if before.Performance != nil && after.Performance != nil {
		result.PerformanceDiff = &VerifyPerfDiff{
			LoadTimeBefore: fmt.Sprintf("%.0fms", before.Performance.Timing.Load),
			LoadTimeAfter:  fmt.Sprintf("%.0fms", after.Performance.Timing.Load),
		}
		if before.Performance.Timing.Load > 0 {
			pctChange := ((after.Performance.Timing.Load - before.Performance.Timing.Load) / before.Performance.Timing.Load) * 100
			if pctChange >= 0 {
				result.PerformanceDiff.Change = fmt.Sprintf("+%.0f%%", pctChange)
			} else {
				result.PerformanceDiff.Change = fmt.Sprintf("%.0f%%", pctChange)
			}
		}
	}

	// Determine verdict
	result.Verdict = vm.determineVerdict(result)

	return result
}

// determineVerdict determines the overall verdict based on changes
func (vm *VerificationManager) determineVerdict(result VerificationResult) string {
	beforeTotal := result.Before.TotalIssues
	afterTotal := result.After.TotalIssues
	hasResolved := len(result.Changes) > 0
	hasNew := len(result.NewIssues) > 0

	// Count actual resolved changes (not just "changed")
	resolvedCount := 0
	for _, c := range result.Changes {
		if c.Type == "resolved" {
			resolvedCount++
		}
	}

	switch {
	case beforeTotal == 0 && afterTotal == 0:
		return "no_issues_detected"
	case resolvedCount > 0 && !hasNew && afterTotal == 0:
		return "fixed"
	case resolvedCount > 0 && !hasNew:
		return "improved"
	case hasResolved && hasNew:
		return "different_issue"
	case hasNew && afterTotal > beforeTotal:
		return "regressed"
	case hasNew:
		return "regressed"
	default:
		return "unchanged"
	}
}

// ============================================
// Helper Functions
// ============================================

// normalizeVerifyErrorMessage normalizes dynamic values in error messages for matching.
// Uses similar patterns to clustering.go but returns placeholders in a test-friendly format.
func normalizeVerifyErrorMessage(msg string) string {
	// Reuse existing patterns from clustering.go
	result := clusterUUIDRegex.ReplaceAllString(msg, "[uuid]")
	result = clusterTimestampRegex.ReplaceAllString(result, "[timestamp]")
	result = clusterNumericIDRegex.ReplaceAllString(result, "[id]")
	// Add file:line normalization not in clustering.go
	result = verifyFileLineRegex.ReplaceAllString(result, "[file]")
	return result
}

// removeFromOrder removes a session ID from the order slice
func (vm *VerificationManager) removeFromOrder(sessionID string) {
	for i, id := range vm.order {
		if id == sessionID {
			newOrder := make([]string, len(vm.order)-1)
			copy(newOrder, vm.order[:i])
			copy(newOrder[i:], vm.order[i+1:])
			vm.order = newOrder
			return
		}
	}
}

// cleanupExpiredLocked removes expired sessions (must hold lock)
func (vm *VerificationManager) cleanupExpiredLocked() {
	now := time.Now()
	expired := make([]string, 0)

	for id, session := range vm.sessions {
		if now.Sub(session.CreatedAt) > vm.ttl {
			expired = append(expired, id)
		}
	}

	for _, id := range expired {
		delete(vm.sessions, id)
		vm.removeFromOrder(id)
	}
}

// ============================================
// MCP Tool Handler
// ============================================

// verifyFixParams defines the MCP tool input schema
type verifyFixParams struct {
	Action    string `json:"action"`
	SessionID string `json:"session_id,omitempty"`
	Label     string `json:"label,omitempty"`
	URLFilter string `json:"url,omitempty"`
}

// HandleTool dispatches the verify_fix MCP tool call
func (vm *VerificationManager) HandleTool(params json.RawMessage) (interface{}, error) {
	var p verifyFixParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	switch p.Action {
	case "start":
		result, err := vm.Start(p.Label, p.URLFilter)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"session_id": result.SessionID,
			"status":     result.Status,
			"label":      result.Label,
			"baseline":   result.Baseline,
		}, nil

	case "watch":
		if p.SessionID == "" {
			return nil, fmt.Errorf("'session_id' is required for watch action")
		}
		result, err := vm.Watch(p.SessionID)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"session_id": result.SessionID,
			"status":     result.Status,
			"message":    result.Message,
		}, nil

	case "compare":
		if p.SessionID == "" {
			return nil, fmt.Errorf("'session_id' is required for compare action")
		}
		result, err := vm.Compare(p.SessionID)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"session_id": result.SessionID,
			"status":     result.Status,
			"label":      result.Label,
			"result": map[string]interface{}{
				"verdict":          result.Result.Verdict,
				"before":           result.Result.Before,
				"after":            result.Result.After,
				"changes":          result.Result.Changes,
				"new_issues":       result.Result.NewIssues,
				"performance_diff": result.Result.PerformanceDiff,
			},
		}, nil

	case "status":
		if p.SessionID == "" {
			return nil, fmt.Errorf("'session_id' is required for status action")
		}
		result, err := vm.Status(p.SessionID)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"session_id": result.SessionID,
			"status":     result.Status,
			"label":      result.Label,
			"created_at": result.CreatedAt,
		}, nil

	case "cancel":
		if p.SessionID == "" {
			return nil, fmt.Errorf("'session_id' is required for cancel action")
		}
		result, err := vm.Cancel(p.SessionID)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"session_id": result.SessionID,
			"status":     result.Status,
		}, nil

	default:
		if p.Action == "" {
			return nil, fmt.Errorf("'action' is required (start, watch, compare, status, cancel)")
		}
		return nil, fmt.Errorf("unknown action %q (valid: start, watch, compare, status, cancel)", p.Action)
	}
}
