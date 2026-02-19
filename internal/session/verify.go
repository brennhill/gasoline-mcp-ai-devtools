// Purpose: Implements session lifecycle, snapshots, and diff state management.
// Docs: docs/features/feature/observe/index.md
// Docs: docs/features/feature/pagination/index.md

// verify.go — Verification Loop (verify_fix) MCP tool.
// Provides before/after session comparison for fix verification.
// AI captures state before a fix, applies fix, captures after, compares to verify.
// Session lifecycle: start -> watch -> compare (or cancel at any point).
package session

import (
	"fmt"
	"regexp"
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
	maxVerificationSessions   = 3
	defaultVerificationTTL    = 30 * time.Minute
	maxBaselineErrors         = 50
	maxBaselineNetworkEntries = 50
)

// ============================================
// Normalization Regex Patterns (verify-specific)
// ============================================

var (
	// UUID pattern for clustering across test runs
	clusterUUIDRegex = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

	// ISO 8601 timestamps for clustering across test runs
	clusterTimestampRegex = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[^\s]*`)

	// Numeric IDs for clustering across test runs
	clusterNumericIDRegex = regexp.MustCompile(`\d{1,19}`)

	// File path with line number: filename.ext:123
	// More specific pattern than clustering.go uses
	verifyFileLineRegex = regexp.MustCompile(`[\w./\\-]+\.[a-zA-Z]{1,4}:\d+`)
)

// ============================================
// Types
// ============================================

// VerificationSession tracks a before/after comparison
type VerificationSession struct {
	ID        string    `json:"verif_session_id"`
	Label     string    `json:"label"`
	Status    string    `json:"status"` // "baseline_captured", "watching", "compared", "cancelled"
	URLFilter string    `json:"url,omitempty"`
	CreatedAt time.Time `json:"created_at"`

	// Baseline snapshot (captured at "start")
	Baseline *VerifSnapshot `json:"baseline"`

	// Watch state
	WatchStartedAt *time.Time `json:"watch_started_at,omitempty"`

	// After snapshot (captured at "compare")
	After *VerifSnapshot `json:"after,omitempty"`
}

// VerifSnapshot is a point-in-time capture of browser state for verification
type VerifSnapshot struct {
	CapturedAt         time.Time            `json:"captured_at"`
	ConsoleErrors      []VerifyError        `json:"console_errors"`
	NetworkErrors      []VerifyNetworkEntry `json:"network_errors"`      // Only 4xx/5xx responses
	AllNetworkRequests []VerifyNetworkEntry `json:"all_network,omitempty"` // All requests (for comparison)
	PageURL            string               `json:"page_url,omitempty"`
	Performance        *performance.PerformanceSnapshot `json:"performance,omitempty"`
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
	VerifSessionID string             `json:"verif_session_id"`
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
	VerifSessionID string `json:"verif_session_id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// CompareResult is the response from the compare action
type CompareResult struct {
	VerifSessionID string             `json:"verif_session_id"`
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
	VerifSessionID string `json:"verif_session_id"`
	Status    string `json:"status"`
}

// StatusResult is the response from the status action
type StatusResult struct {
	VerifSessionID string    `json:"verif_session_id"`
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
		VerifSessionID: sessionID,
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
		VerifSessionID: sessionID,
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
		VerifSessionID: sessionID,
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
		VerifSessionID: sessionID,
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
		VerifSessionID: sessionID,
		Label:     session.Label,
		Status:    session.Status,
		CreatedAt: session.CreatedAt,
	}, nil
}

// ============================================
// Snapshot Capture
// ============================================

// convertConsoleErrors converts snapshot errors to verification errors.
func convertConsoleErrors(errors []SnapshotError) []VerifyError {
	result := make([]VerifyError, 0, len(errors))
	for _, e := range errors {
		result = append(result, VerifyError{
			Message:    e.Message,
			Normalized: normalizeVerifyErrorMessage(e.Message),
			Count:      e.Count,
		})
	}
	return truncateSlice(result, maxBaselineErrors)
}

// convertNetworkRequests converts and filters network requests, returning all requests and error-only requests.
func convertNetworkRequests(network []SnapshotNetworkRequest, urlFilter string) ([]VerifyNetworkEntry, []VerifyNetworkEntry) {
	allNetwork := make([]VerifyNetworkEntry, 0, len(network))
	networkErrors := make([]VerifyNetworkEntry, 0)
	for _, req := range network {
		if urlFilter != "" && !strings.Contains(req.URL, urlFilter) {
			continue
		}
		entry := VerifyNetworkEntry{
			Method: req.Method, URL: req.URL,
			Path: capture.ExtractURLPath(req.URL), Status: req.Status, Duration: req.Duration,
		}
		allNetwork = append(allNetwork, entry)
		if req.Status >= 400 {
			networkErrors = append(networkErrors, entry)
		}
	}
	return truncateSlice(allNetwork, maxBaselineNetworkEntries),
		truncateSlice(networkErrors, maxBaselineNetworkEntries)
}

// truncateSlice returns the first maxLen elements of a slice, or the full slice if shorter.
func truncateSlice[T any](s []T, maxLen int) []T {
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}

// captureSnapshot captures current state from the reader
func (vm *VerificationManager) captureSnapshot(urlFilter string) *VerifSnapshot {
	perf := vm.reader.GetPerformance()
	allNetwork, networkErrors := convertNetworkRequests(vm.reader.GetNetworkRequests(), urlFilter)

	var perfCopy *performance.PerformanceSnapshot
	if perf != nil {
		p := *perf
		perfCopy = &p
	}

	return &VerifSnapshot{
		CapturedAt:         time.Now(),
		ConsoleErrors:      convertConsoleErrors(vm.reader.GetConsoleErrors()),
		NetworkErrors:      networkErrors,
		AllNetworkRequests: allNetwork,
		PageURL:            vm.reader.GetCurrentPageURL(),
		Performance:        perfCopy,
	}
}

// buildBaselineSummary creates a summary of the baseline snapshot
func (vm *VerificationManager) buildBaselineSummary(baseline *VerifSnapshot) BaselineSummary {
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
// Helper Functions
// ============================================

// normalizeVerifyErrorMessage normalizes dynamic values in error messages for matching.
// Uses similar patterns to clustering.go but returns placeholders in a test-friendly format.
func normalizeVerifyErrorMessage(msg string) string {
	// Order matters: apply more specific patterns first
	// File:line must be matched before numeric IDs, or the line number gets replaced first
	result := clusterUUIDRegex.ReplaceAllString(msg, "[uuid]")
	result = clusterTimestampRegex.ReplaceAllString(result, "[timestamp]")
	result = verifyFileLineRegex.ReplaceAllString(result, "[file]")
	result = clusterNumericIDRegex.ReplaceAllString(result, "[id]")
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
