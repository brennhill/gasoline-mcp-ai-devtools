// alerts.go — AlertBuffer methods and pure alert processing functions.
package streaming

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/types"
)

// ============================================
// AlertBuffer Methods
// ============================================

// AddAlert appends an alert to the buffer, evicting the oldest if at capacity.
// Also emits the alert as an MCP notification if streaming is enabled.
func (ab *AlertBuffer) AddAlert(a types.Alert) {
	ab.Mu.Lock()
	if len(ab.Alerts) >= AlertBufferCap {
		newAlerts := make([]types.Alert, len(ab.Alerts)-1)
		copy(newAlerts, ab.Alerts[1:])
		ab.Alerts = newAlerts
	}
	ab.Alerts = append(ab.Alerts, a)
	ab.Mu.Unlock()

	if ab.Stream != nil {
		ab.Stream.EmitAlert(a)
	}
}

// DrainAlerts returns all pending alerts (deduplicated, correlated, sorted)
// and clears the buffer. Returns nil if no alerts pending.
func (ab *AlertBuffer) DrainAlerts() []types.Alert {
	ab.Mu.Lock()
	if len(ab.Alerts) == 0 {
		ab.Mu.Unlock()
		return nil
	}
	raw := make([]types.Alert, len(ab.Alerts))
	copy(raw, ab.Alerts)
	ab.Alerts = nil
	ab.Mu.Unlock()

	deduped := DeduplicateAlerts(raw)
	correlated := CorrelateAlerts(deduped)
	SortAlertsByPriority(correlated)
	return correlated
}

// ============================================
// CI Result Processing
// ============================================

// ProcessCIResult stores the CI result and generates an alert if new.
// Returns the new alert (for streaming), or nil if this was an idempotent update.
func (ab *AlertBuffer) ProcessCIResult(ciResult types.CIResult) *types.Alert {
	ab.Mu.Lock()
	defer ab.Mu.Unlock()

	if ab.updateExistingCIResult(ciResult) {
		return nil
	}

	if len(ab.CIResults) >= CIResultsCap {
		newResults := make([]types.CIResult, len(ab.CIResults)-1)
		copy(newResults, ab.CIResults[1:])
		ab.CIResults = newResults
	}
	ab.CIResults = append(ab.CIResults, ciResult)

	alert := BuildCIAlert(ciResult)
	if len(ab.Alerts) >= AlertBufferCap {
		newAlerts := make([]types.Alert, len(ab.Alerts)-1)
		copy(newAlerts, ab.Alerts[1:])
		ab.Alerts = newAlerts
	}
	ab.Alerts = append(ab.Alerts, alert)
	return &alert
}

// updateExistingCIResult checks for and updates an existing CI result.
// Must be called with ab.Mu held. Returns true if an existing entry was updated.
func (ab *AlertBuffer) updateExistingCIResult(ciResult types.CIResult) bool {
	for i := range ab.CIResults {
		if ab.CIResults[i].Commit != ciResult.Commit || ab.CIResults[i].Status != ciResult.Status {
			continue
		}
		ab.CIResults[i] = ciResult
		for j, alert := range ab.Alerts {
			if alert.Category == "ci" && strings.Contains(alert.Detail, ciResult.Commit) {
				ab.Alerts[j] = BuildCIAlert(ciResult)
				break
			}
		}
		return true
	}
	return false
}

// BuildCIAlert creates an alert from a CI result.
func BuildCIAlert(ci types.CIResult) types.Alert {
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

	return types.Alert{
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

// RecordErrorForAnomaly tracks error timestamps for anomaly detection.
// If error frequency exceeds 3x the rolling average in a 10-second window,
// an anomaly alert is generated.
func (ab *AlertBuffer) RecordErrorForAnomaly(t time.Time) {
	ab.Mu.Lock()

	ab.ErrorTimes = append(ab.ErrorTimes, t)
	ab.pruneErrorTimes(t)

	if len(ab.ErrorTimes) < 2 {
		ab.Mu.Unlock()
		return
	}

	recentCount := ab.countRecentErrors(t)
	rollingAvg := float64(len(ab.ErrorTimes)) / (float64(AnomalyWindowSeconds) / float64(AnomalyBucketSeconds))

	newAlert := ab.maybeCreateAnomalyAlert(t, recentCount, rollingAvg)

	ab.Mu.Unlock()

	if newAlert != nil && ab.Stream != nil {
		ab.Stream.EmitAlert(*newAlert)
	}
}

// pruneErrorTimes removes timestamps older than the anomaly window.
// Must be called with ab.Mu held.
func (ab *AlertBuffer) pruneErrorTimes(t time.Time) {
	cutoff := t.Add(-time.Duration(AnomalyWindowSeconds) * time.Second)
	pruned := make([]time.Time, 0, len(ab.ErrorTimes))
	for _, et := range ab.ErrorTimes {
		if et.After(cutoff) {
			pruned = append(pruned, et)
		}
	}
	ab.ErrorTimes = pruned
}

// countRecentErrors counts errors within the anomaly bucket window.
// Must be called with ab.Mu held.
func (ab *AlertBuffer) countRecentErrors(t time.Time) int {
	recentCutoff := t.Add(-time.Duration(AnomalyBucketSeconds) * time.Second)
	count := 0
	for _, et := range ab.ErrorTimes {
		if et.After(recentCutoff) {
			count++
		}
	}
	return count
}

// maybeCreateAnomalyAlert checks for a spike and creates an alert if warranted.
// Must be called with ab.Mu held. Returns the new alert or nil.
func (ab *AlertBuffer) maybeCreateAnomalyAlert(t time.Time, recentCount int, rollingAvg float64) *types.Alert {
	if rollingAvg <= 0 || float64(recentCount) <= 3.0*rollingAvg {
		return nil
	}

	if ab.hasRecentAnomalyAlert(t) {
		return nil
	}

	alert := types.Alert{
		Severity:  "warning",
		Category:  "anomaly",
		Title:     "Error frequency spike detected",
		Detail:    fmt.Sprintf("%d errors in last %ds vs %.1f rolling average", recentCount, AnomalyBucketSeconds, rollingAvg),
		Timestamp: t.Format(time.RFC3339),
		Source:    "anomaly_detector",
	}
	if len(ab.Alerts) >= AlertBufferCap {
		newAlerts := make([]types.Alert, len(ab.Alerts)-1)
		copy(newAlerts, ab.Alerts[1:])
		ab.Alerts = newAlerts
	}
	ab.Alerts = append(ab.Alerts, alert)
	return &alert
}

// hasRecentAnomalyAlert checks if an anomaly alert was already generated recently.
// Must be called with ab.Mu held.
func (ab *AlertBuffer) hasRecentAnomalyAlert(t time.Time) bool {
	for _, a := range ab.Alerts {
		if a.Category != "anomaly" || a.Source != "anomaly_detector" {
			continue
		}
		at, err := time.Parse(time.RFC3339, a.Timestamp)
		if err != nil {
			continue
		}
		if t.Sub(at) < time.Duration(AnomalyBucketSeconds)*time.Second {
			return true
		}
	}
	return false
}

// ============================================
// Alert Processing: Deduplication
// ============================================

// DeduplicateAlerts merges alerts with the same title+category.
func DeduplicateAlerts(alerts []types.Alert) []types.Alert {
	type dedupKey struct {
		title    string
		category string
	}

	seen := make(map[dedupKey]int)
	var result []types.Alert

	for _, a := range alerts {
		key := dedupKey{title: a.Title, category: a.Category}
		if idx, ok := seen[key]; ok {
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

// CorrelateAlerts groups regression+anomaly pairs within 5s.
func CorrelateAlerts(alerts []types.Alert) []types.Alert {
	if len(alerts) < 2 {
		return alerts
	}

	var result []types.Alert
	used := make([]bool, len(alerts))

	for i := 0; i < len(alerts); i++ {
		if used[i] {
			continue
		}
		if partner := findCorrelationPartner(alerts, used, i); partner >= 0 {
			result = append(result, MergeAlerts(alerts[i], alerts[partner]))
			used[i] = true
			used[partner] = true
		} else {
			result = append(result, alerts[i])
			used[i] = true
		}
	}

	return result
}

func findCorrelationPartner(alerts []types.Alert, used []bool, i int) int {
	if alerts[i].Category != "regression" && alerts[i].Category != "anomaly" {
		return -1
	}
	for j := i + 1; j < len(alerts); j++ {
		if !used[j] && CanCorrelate(alerts[i], alerts[j]) {
			return j
		}
	}
	return -1
}

// CanCorrelate checks if two alerts can be correlated (regression+anomaly within window).
func CanCorrelate(a, b types.Alert) bool {
	if (a.Category != "regression" || b.Category != "anomaly") &&
		(a.Category != "anomaly" || b.Category != "regression") {
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
	return diff <= CorrelationWindow
}

// MergeAlerts combines two correlated alerts into one.
func MergeAlerts(a, b types.Alert) types.Alert {
	severity := a.Severity
	if SeverityRank(b.Severity) > SeverityRank(a.Severity) {
		severity = b.Severity
	}

	ts := a.Timestamp
	if b.Timestamp > a.Timestamp {
		ts = b.Timestamp
	}

	return types.Alert{
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

// SortAlertsByPriority sorts alerts by severity (descending) then timestamp (newest first).
func SortAlertsByPriority(alerts []types.Alert) {
	sort.SliceStable(alerts, func(i, j int) bool {
		ri := SeverityRank(alerts[i].Severity)
		rj := SeverityRank(alerts[j].Severity)
		if ri != rj {
			return ri > rj
		}
		return alerts[i].Timestamp > alerts[j].Timestamp
	})
}

// SeverityRank returns the numeric rank of a severity string.
func SeverityRank(s string) int {
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
// Alert Formatting
// ============================================

// FormatAlertsBlock produces the text content block for alerts.
func FormatAlertsBlock(alerts []types.Alert) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("--- ALERTS (%d) ---\n", len(alerts)))

	if len(alerts) > 3 {
		summary := BuildAlertSummary(alerts)
		sb.WriteString(summary)
		sb.WriteString("\n")
	}

	alertsJSON, _ := json.Marshal(alerts)
	sb.Write(alertsJSON)

	return sb.String()
}

// BuildAlertSummary creates a one-line summary like "4 alerts: 1 regression, 2 anomaly, 1 ci".
func BuildAlertSummary(alerts []types.Alert) string {
	categories := make(map[string]int)
	for _, a := range alerts {
		categories[a.Category]++
	}

	var parts []string
	for _, cat := range []string{"regression", "anomaly", "ci", "noise", "threshold"} {
		if count, ok := categories[cat]; ok {
			parts = append(parts, fmt.Sprintf("%d %s", count, cat))
		}
	}

	return fmt.Sprintf("%d alerts: %s", len(alerts), strings.Join(parts, ", "))
}
