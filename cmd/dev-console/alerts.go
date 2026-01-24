// alerts.go — Push-based alert system for proactive error notification.
// Monitors incoming browser events and surfaces new errors, network failures,
// and performance regressions via the MCP tool response metadata.
// Design: Alert buffer with deduplication and delivery tracking. Alerts are
// appended to observe/analyze responses so the AI learns about issues without
// needing to poll. Configurable severity thresholds.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// ============================================
// Push-Based Alerts
// ============================================

const (
	alertBufferCap = 50
	ciResultsCap   = 10
	// Correlation window: alerts within this duration are candidates for grouping
	correlationWindow = 5 * time.Second
)

// Alert represents a server-generated alert that piggybacks on observe responses.
type Alert struct {
	Severity  string `json:"severity"`         // "info", "warning", "error"
	Category  string `json:"category"`         // "regression", "anomaly", "ci", "noise", "threshold"
	Title     string `json:"title"`            // Short summary
	Detail    string `json:"detail,omitempty"` // Longer explanation
	Timestamp string `json:"timestamp"`        // ISO 8601
	Source    string `json:"source"`           // What generated it
	Count     int    `json:"count,omitempty"`  // Deduplication count (>1 means repeated)
}

// CIResult stores a CI/CD webhook result.
type CIResult struct {
	Status     string      `json:"status"`      // "success", "failure", "error"
	Source     string      `json:"source"`      // "github-actions", "gitlab-ci", "custom"
	Ref        string      `json:"ref"`         // Branch ref
	Commit     string      `json:"commit"`      // Commit SHA
	Summary    string      `json:"summary"`     // Human-readable summary
	Failures   []CIFailure `json:"failures"`    // Failed tests
	URL        string      `json:"url"`         // Link to CI run
	DurationMs int         `json:"duration_ms"` // Build duration
	ReceivedAt time.Time   `json:"-"`           // When we received it
}

// CIFailure represents a single test failure in a CI result.
type CIFailure struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// AlertBuffer holds the alert state on ToolHandler.
// Separated from other mutexes to avoid lock ordering issues.
type AlertBuffer struct {
	alertMu   sync.Mutex
	alerts    []Alert
	ciResults []CIResult
	// Anomaly detection: sliding window error counter
	errorTimes []time.Time
}

// addAlert appends an alert to the buffer, evicting the oldest if at capacity.
func (h *ToolHandler) addAlert(a Alert) {
	h.alertMu.Lock()
	defer h.alertMu.Unlock()

	if len(h.alerts) >= alertBufferCap {
		// FIFO eviction: remove oldest
		h.alerts = h.alerts[1:]
	}
	h.alerts = append(h.alerts, a)
}

// drainAlerts returns all pending alerts (processed: deduped, sorted, correlated)
// and clears the buffer. Returns nil if no alerts pending.
func (h *ToolHandler) drainAlerts() []Alert {
	h.alertMu.Lock()
	if len(h.alerts) == 0 {
		h.alertMu.Unlock()
		return nil
	}
	raw := make([]Alert, len(h.alerts))
	copy(raw, h.alerts)
	h.alerts = h.alerts[:0]
	h.alertMu.Unlock()

	// Step 1: Deduplicate (same title + category → merge with count)
	deduped := deduplicateAlerts(raw)

	// Step 2: Correlate (regression + anomaly within 5s → compound alert)
	correlated := correlateAlerts(deduped)

	// Step 3: Sort by priority (error > warning > info, then newest first)
	sortAlertsByPriority(correlated)

	return correlated
}

// formatAlertsBlock produces the text content block for alerts.
func formatAlertsBlock(alerts []Alert) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("--- ALERTS (%d) ---\n", len(alerts)))

	// Summary prefix when 4+ alerts
	if len(alerts) > 3 {
		summary := buildAlertSummary(alerts)
		sb.WriteString(summary)
		sb.WriteString("\n")
	}

	alertsJSON, _ := json.Marshal(alerts)
	sb.Write(alertsJSON)

	return sb.String()
}

// buildAlertSummary creates a one-line summary like "4 alerts: 1 regression, 2 anomaly, 1 ci"
func buildAlertSummary(alerts []Alert) string {
	categories := make(map[string]int)
	for _, a := range alerts {
		categories[a.Category]++
	}

	var parts []string
	// Ordered output for determinism
	for _, cat := range []string{"regression", "anomaly", "ci", "noise", "threshold"} {
		if count, ok := categories[cat]; ok {
			parts = append(parts, fmt.Sprintf("%d %s", count, cat))
		}
	}

	return fmt.Sprintf("%d alerts: %s", len(alerts), strings.Join(parts, ", "))
}

// ============================================
// Alert Processing: Deduplication
// ============================================

func deduplicateAlerts(alerts []Alert) []Alert {
	type dedupKey struct {
		title    string
		category string
	}

	seen := make(map[dedupKey]int) // key → index in result
	var result []Alert

	for _, a := range alerts {
		key := dedupKey{title: a.Title, category: a.Category}
		if idx, ok := seen[key]; ok {
			// Merge: increment count, keep latest timestamp
			result[idx].Count++
			if a.Timestamp > result[idx].Timestamp {
				result[idx].Timestamp = a.Timestamp
			}
		} else {
			a.Count = 1
			seen[key] = len(result)
			result = append(result, a)
		}
	}

	// Remove count=1 (no duplication) to keep output clean
	for i := range result {
		if result[i].Count == 1 {
			result[i].Count = 0
		}
	}

	return result
}

// ============================================
// Alert Processing: Correlation
// ============================================

func correlateAlerts(alerts []Alert) []Alert {
	if len(alerts) < 2 {
		return alerts
	}

	var result []Alert
	used := make([]bool, len(alerts))

	for i := 0; i < len(alerts); i++ {
		if used[i] {
			continue
		}

		// Look for correlation partner: regression + anomaly within 5s
		if alerts[i].Category == "regression" || alerts[i].Category == "anomaly" {
			for j := i + 1; j < len(alerts); j++ {
				if used[j] {
					continue
				}
				if canCorrelate(alerts[i], alerts[j]) {
					// Merge into compound alert
					compound := mergeAlerts(alerts[i], alerts[j])
					result = append(result, compound)
					used[i] = true
					used[j] = true
					break
				}
			}
		}

		if !used[i] {
			result = append(result, alerts[i])
			used[i] = true
		}
	}

	return result
}

func canCorrelate(a, b Alert) bool {
	// Only correlate regression + anomaly pairs
	if !((a.Category == "regression" && b.Category == "anomaly") ||
		(a.Category == "anomaly" && b.Category == "regression")) {
		return false
	}

	ta, errA := time.Parse(time.RFC3339, a.Timestamp)
	tb, errB := time.Parse(time.RFC3339, b.Timestamp)
	if errA != nil || errB != nil {
		return false
	}

	diff := ta.Sub(tb)
	if diff < 0 {
		diff = -diff
	}
	return diff <= correlationWindow
}

func mergeAlerts(a, b Alert) Alert {
	// Use higher severity
	severity := a.Severity
	if severityRank(b.Severity) > severityRank(a.Severity) {
		severity = b.Severity
	}

	// Latest timestamp
	ts := a.Timestamp
	if b.Timestamp > a.Timestamp {
		ts = b.Timestamp
	}

	return Alert{
		Severity:  severity,
		Category:  "regression",
		Title:     "Correlated: " + a.Title + " + " + b.Title,
		Detail:    a.Detail + " | " + b.Detail,
		Timestamp: ts,
		Source:    a.Source,
	}
}

// ============================================
// Alert Processing: Priority Sorting
// ============================================

func sortAlertsByPriority(alerts []Alert) {
	sort.SliceStable(alerts, func(i, j int) bool {
		ri := severityRank(alerts[i].Severity)
		rj := severityRank(alerts[j].Severity)
		if ri != rj {
			return ri > rj // Higher rank first
		}
		// Same severity: newest first
		return alerts[i].Timestamp > alerts[j].Timestamp
	})
}

func severityRank(s string) int {
	switch s {
	case "error":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

// ============================================
// CI/CD Webhook Handler
// ============================================

func (h *ToolHandler) handleCIWebhook(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 1MB
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error":"request body too large"}`, http.StatusBadRequest)
		return
	}

	var ciResult CIResult
	if err := json.Unmarshal(body, &ciResult); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	ciResult.ReceivedAt = time.Now().UTC()

	h.alertMu.Lock()

	// Idempotency: check if same commit+status already exists
	updated := false
	for i := range h.ciResults {
		if h.ciResults[i].Commit == ciResult.Commit && h.ciResults[i].Status == ciResult.Status {
			h.ciResults[i] = ciResult
			updated = true
			// Also update the corresponding alert
			for j, alert := range h.alerts {
				if alert.Category == "ci" && strings.Contains(alert.Detail, ciResult.Commit) {
					h.alerts[j] = h.buildCIAlert(ciResult)
					break
				}
			}
			break
		}
	}

	if !updated {
		// Cap CI results at 10
		if len(h.ciResults) >= ciResultsCap {
			h.ciResults = h.ciResults[1:]
		}
		h.ciResults = append(h.ciResults, ciResult)

		// Generate alert (append without lock since we hold it)
		alert := h.buildCIAlert(ciResult)
		if len(h.alerts) >= alertBufferCap {
			h.alerts = h.alerts[1:]
		}
		h.alerts = append(h.alerts, alert)
	}

	h.alertMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (h *ToolHandler) buildCIAlert(ci CIResult) Alert {
	severity := "info"
	if ci.Status == "failure" || ci.Status == "error" {
		severity = "error"
	}

	detail := ci.Summary
	if len(ci.Failures) > 0 {
		failNames := make([]string, 0, len(ci.Failures))
		for _, f := range ci.Failures {
			failNames = append(failNames, f.Name)
		}
		detail += " | Failed: " + strings.Join(failNames, ", ")
	}
	if ci.Commit != "" {
		detail += " [" + ci.Commit + "]"
	}

	return Alert{
		Severity:  severity,
		Category:  "ci",
		Title:     fmt.Sprintf("CI %s (%s)", ci.Status, ci.Source),
		Detail:    detail,
		Timestamp: ci.ReceivedAt.Format(time.RFC3339),
		Source:    "ci_webhook",
	}
}

// ============================================
// Anomaly Detection
// ============================================

// recordErrorForAnomaly tracks error timestamps for anomaly detection.
// If error frequency exceeds 3x the rolling average in a 10-second window,
// an anomaly alert is generated.
func (h *ToolHandler) recordErrorForAnomaly(t time.Time) {
	h.alertMu.Lock()
	defer h.alertMu.Unlock()

	h.errorTimes = append(h.errorTimes, t)

	// Prune entries older than 60 seconds
	cutoff := t.Add(-60 * time.Second)
	pruned := h.errorTimes[:0]
	for _, et := range h.errorTimes {
		if et.After(cutoff) {
			pruned = append(pruned, et)
		}
	}
	h.errorTimes = pruned

	// Need at least 2 data points to compute a rate
	if len(h.errorTimes) < 2 {
		return
	}

	// Count errors in last 10 seconds
	recentCutoff := t.Add(-10 * time.Second)
	recentCount := 0
	for _, et := range h.errorTimes {
		if et.After(recentCutoff) {
			recentCount++
		}
	}

	// Rolling average: total errors in 60s window / 6 (number of 10s buckets)
	totalCount := len(h.errorTimes)
	rollingAvg := float64(totalCount) / 6.0

	// Spike detection: >3x the rolling average
	if rollingAvg > 0 && float64(recentCount) > 3.0*rollingAvg {
		// Check we haven't already generated an anomaly alert recently
		alreadyAlerted := false
		for _, a := range h.alerts {
			if a.Category == "anomaly" && a.Source == "anomaly_detector" {
				// Only skip if alert is from the last 10 seconds
				if at, err := time.Parse(time.RFC3339, a.Timestamp); err == nil {
					if t.Sub(at) < 10*time.Second {
						alreadyAlerted = true
						break
					}
				}
			}
		}

		if !alreadyAlerted {
			alert := Alert{
				Severity:  "warning",
				Category:  "anomaly",
				Title:     "Error frequency spike detected",
				Detail:    fmt.Sprintf("%d errors in last 10s vs %.1f rolling average", recentCount, rollingAvg),
				Timestamp: t.Format(time.RFC3339),
				Source:    "anomaly_detector",
			}
			if len(h.alerts) >= alertBufferCap {
				h.alerts = h.alerts[1:]
			}
			h.alerts = append(h.alerts, alert)
		}
	}
}
