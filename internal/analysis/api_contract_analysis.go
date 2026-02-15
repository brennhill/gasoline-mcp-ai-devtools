// api_contract_analysis.go — Contract analysis, reporting, and violation detection.
// Contains the analysis pipeline that processes tracked endpoints, detects
// error spikes, compares response shapes, and generates reports.
package analysis

import (
	"encoding/json"
	"fmt"
	"github.com/dev-console/dev-console/internal/capture"
	"sort"
	"strings"
	"time"
)

// detectErrorSpike checks if there's a pattern of success followed by errors.
func (v *APIContractValidator) detectErrorSpike(tracker *EndpointTracker, body capture.NetworkBody) *APIContractViolation {
	history := tracker.StatusHistory
	if len(history) < 3 {
		return nil
	}

	// Look at recent history for pattern: successes followed by errors
	// History includes the current request (just added)
	recentErrors := 0
	earlierSuccesses := 0

	// Count consecutive recent errors (including current)
	for i := len(history) - 1; i >= 0; i-- {
		if history[i] >= 400 {
			recentErrors++
		} else {
			break
		}
	}

	// Count successes before the error streak
	for i := len(history) - 1 - recentErrors; i >= 0 && i >= len(history)-10; i-- {
		if history[i] >= 200 && history[i] < 300 {
			earlierSuccesses++
		}
	}

	// Detect spike: had successes, now consecutive errors
	if body.Status >= 500 && earlierSuccesses >= 2 && recentErrors >= 2 {
		var errorBody map[string]any
		_ = json.Unmarshal([]byte(body.ResponseBody), &errorBody)

		return &APIContractViolation{
			Endpoint:      tracker.Endpoint,
			Type:          "error_spike",
			Description:   fmt.Sprintf("Endpoint returned success %d times, then started returning errors", earlierSuccesses),
			StatusHistory: history,
			LastErrorBody: errorBody,
		}
	}

	return nil
}

// compareShapes compares expected vs actual shape and returns violations.
func (v *APIContractValidator) compareShapes(endpoint string, expected, actual, actualData any) []APIContractViolation {
	expectedMap, eOK := expected.(map[string]any)
	actualMap, aOK := actual.(map[string]any)

	if !eOK || !aOK {
		return detectTopLevelTypeChange(endpoint, expected, actual)
	}

	var violations []APIContractViolation
	violations = append(violations, detectMissingFields(endpoint, expectedMap, actualMap)...)
	violations = append(violations, detectNewFields(endpoint, expectedMap, actualMap)...)
	violations = append(violations, detectFieldTypeChanges(endpoint, expectedMap, actualMap, actualData)...)
	return violations
}

// detectTopLevelTypeChange reports a violation if the top-level response type changed.
func detectTopLevelTypeChange(endpoint string, expected, actual any) []APIContractViolation {
	if fmt.Sprintf("%T", expected) == fmt.Sprintf("%T", actual) {
		return nil
	}
	return []APIContractViolation{{
		Endpoint:     endpoint,
		Type:         "type_change",
		Description:  "Response type changed",
		ExpectedType: describeType(expected),
		ActualType:   describeType(actual),
	}}
}

// detectMissingFields returns a shape_change violation if fields from the expected shape are missing.
func detectMissingFields(endpoint string, expectedMap, actualMap map[string]any) []APIContractViolation {
	var missing []string
	for field := range expectedMap {
		if field == "$array" {
			continue
		}
		if _, found := actualMap[field]; !found {
			missing = append(missing, field)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return []APIContractViolation{{
		Endpoint:      endpoint,
		Type:          "shape_change",
		Description:   fmt.Sprintf("Field(s) missing from response: %s", strings.Join(missing, ", ")),
		MissingFields: missing,
		ExpectedShape: toStringMap(expectedMap),
		ActualShape:   toStringMap(actualMap),
	}}
}

// detectNewFields returns a new_field violation if unexpected fields appeared.
func detectNewFields(endpoint string, expectedMap, actualMap map[string]any) []APIContractViolation {
	var newFields []string
	for field := range actualMap {
		if field == "$array" {
			continue
		}
		if _, found := expectedMap[field]; !found {
			newFields = append(newFields, field)
		}
	}
	if len(newFields) == 0 {
		return nil
	}
	sort.Strings(newFields)
	return []APIContractViolation{{
		Endpoint:    endpoint,
		Type:        "new_field",
		Description: fmt.Sprintf("New field(s) appeared in response: %s", strings.Join(newFields, ", ")),
		NewFields:   newFields,
	}}
}

// detectFieldTypeChanges returns violations for fields whose types changed or became null.
func detectFieldTypeChanges(endpoint string, expectedMap, actualMap map[string]any, actualData any) []APIContractViolation {
	actualDataMap, _ := actualData.(map[string]any)
	var violations []APIContractViolation
	for field, expectedType := range expectedMap {
		actualType, found := actualMap[field]
		if !found {
			continue
		}
		expectedStr := describeType(expectedType)
		actualStr := describeType(actualType)

		if v := classifyFieldTypeChange(endpoint, field, expectedStr, actualStr, actualDataMap); v != nil {
			violations = append(violations, *v)
		}
	}
	return violations
}

// classifyFieldTypeChange returns a violation if a field's type changed, or nil.
func classifyFieldTypeChange(endpoint, field, expectedType, actualType string, actualDataMap map[string]any) *APIContractViolation {
	if expectedType == actualType {
		return nil
	}
	if actualType == "null" && expectedType != "null" {
		return &APIContractViolation{
			Endpoint: endpoint, Type: "null_field",
			Description:  fmt.Sprintf("Field '%s' became null (was %s)", field, expectedType),
			Field:        field,
			ExpectedType: expectedType, ActualType: "null",
		}
	}
	if actualType != "null" {
		var sampleValue any
		if actualDataMap != nil {
			sampleValue = actualDataMap[field]
		}
		return &APIContractViolation{
			Endpoint: endpoint, Type: "type_change",
			Description:  fmt.Sprintf("Field '%s' changed type from %s to %s", field, expectedType, actualType),
			Field:        field,
			ExpectedType: expectedType, ActualType: actualType,
			SampleValue: sampleValue,
		}
	}
	return nil
}

func (v *APIContractValidator) addViolation(tracker *EndpointTracker, violation APIContractViolation) {
	// Enrich violation with metadata
	now := time.Now().Format(time.RFC3339)
	violation.ViolationType = violation.Type
	violation.Severity = violationSeverity(violation.Type)
	violation.AffectedCallCount = 1

	// Check for existing violation of same type to update timestamps
	for i := range tracker.Violations {
		if tracker.Violations[i].Type == violation.Type && tracker.Violations[i].Endpoint == violation.Endpoint {
			// Update existing: bump count and lastSeenAt
			tracker.Violations[i].AffectedCallCount++
			tracker.Violations[i].LastSeenAt = now
			return
		}
	}

	// New violation: set firstSeenAt and lastSeenAt
	violation.FirstSeenAt = now
	violation.LastSeenAt = now

	if len(tracker.Violations) >= maxViolationsPerEndpoint {
		newViolations := make([]APIContractViolation, len(tracker.Violations)-1)
		copy(newViolations, tracker.Violations[1:])
		tracker.Violations = newViolations
	}
	tracker.Violations = append(tracker.Violations, violation)
}

// violationSeverity maps violation types to severity levels.
func violationSeverity(violationType string) string {
	switch violationType {
	case "error_spike":
		return "critical"
	case "shape_change":
		return "high"
	case "type_change":
		return "high"
	case "null_field":
		return "medium"
	case "new_field":
		return "low"
	default:
		return "medium"
	}
}

// ============================================
// MCP Tool Interface
// ============================================

// analyzeAccum holds intermediate totals during endpoint analysis.
type analyzeAccum struct {
	violations       []APIContractViolation
	totalRequests    int
	cleanEndpoints   int
	trackedEndpoints int
	earliestCall     time.Time
}

// Analyze processes tracked endpoints and returns all violations.
func (v *APIContractValidator) Analyze(filter APIContractFilter) APIContractAnalyzeResult {
	v.mu.RLock()
	defer v.mu.RUnlock()

	acc := v.accumulateViolations(filter)

	sort.Slice(acc.violations, func(i, j int) bool {
		return acc.violations[i].Endpoint < acc.violations[j].Endpoint
	})

	return v.buildAnalyzeResult(acc, filter)
}

// accumulateViolations collects violations and statistics from all tracked endpoints.
func (v *APIContractValidator) accumulateViolations(filter APIContractFilter) analyzeAccum {
	var acc analyzeAccum
	for endpoint, tracker := range v.trackers {
		if !v.matchesFilter(endpoint, filter) {
			continue
		}
		acc.trackedEndpoints++
		acc.totalRequests += tracker.CallCount
		if !tracker.FirstCalled.IsZero() && (acc.earliestCall.IsZero() || tracker.FirstCalled.Before(acc.earliestCall)) {
			acc.earliestCall = tracker.FirstCalled
		}
		if len(tracker.Violations) > 0 {
			acc.violations = append(acc.violations, tracker.Violations...)
		} else {
			acc.cleanEndpoints++
		}
	}
	return acc
}

// buildAnalyzeResult constructs the final analysis result from accumulated data.
func (v *APIContractValidator) buildAnalyzeResult(acc analyzeAccum, filter APIContractFilter) APIContractAnalyzeResult {
	result := APIContractAnalyzeResult{
		Action:                "analyzed",
		AnalyzedAt:            time.Now().Format(time.RFC3339),
		Violations:            acc.violations,
		TrackedEndpoints:      acc.trackedEndpoints,
		TotalRequestsAnalyzed: acc.totalRequests,
		CleanEndpoints:        acc.cleanEndpoints,
		Summary: &AnalyzeSummary{
			Violations: len(acc.violations), Endpoints: acc.trackedEndpoints,
			TotalRequests: acc.totalRequests, CleanEndpoints: acc.cleanEndpoints,
		},
		PossibleViolationTypes: []string{"shape_change", "type_change", "error_spike", "new_field", "null_field"},
	}
	if !acc.earliestCall.IsZero() {
		result.DataWindowStartedAt = acc.earliestCall.Format(time.RFC3339)
	}
	if filter.URLFilter != "" || len(filter.IgnoreEndpoints) > 0 {
		result.AppliedFilter = &AppliedFilterEcho{URL: filter.URLFilter, IgnoreEndpoints: filter.IgnoreEndpoints}
	}
	if len(acc.violations) == 0 {
		result.Hint = analyzeHint(acc.trackedEndpoints)
	}
	return result
}

// analyzeHint returns a helpful hint when no violations are found.
func analyzeHint(trackedEndpoints int) string {
	if trackedEndpoints > 0 {
		return fmt.Sprintf("No violations detected. All %d tracked endpoint(s) have consistent response shapes.", trackedEndpoints)
	}
	return "No violations detected. No endpoints tracked yet — browse your application to capture API traffic."
}

// Report returns the current state of all tracked endpoint schemas.
func (v *APIContractValidator) Report(filter APIContractFilter) APIContractReportResult {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var endpoints []EndpointContractReport
	for endpoint, tracker := range v.trackers {
		if !v.matchesFilter(endpoint, filter) {
			continue
		}
		endpoints = append(endpoints, buildEndpointReport(endpoint, tracker))
	}

	// Sort by call count (most used first)
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].CallCount > endpoints[j].CallCount
	})

	result := APIContractReportResult{
		Action:     "report",
		AnalyzedAt: time.Now().Format(time.RFC3339),
		Endpoints:  endpoints,
		ConsistencyLevels: map[string]string{
			"1.0":       "Perfect — all responses match established schema",
			"0.9-0.99":  "Good — occasional minor deviations",
			"0.7-0.89":  "Degraded — frequent schema mismatches, investigate",
			"below 0.7": "Poor — endpoint contract is unstable",
		},
	}

	// appliedFilter echo
	if filter.URLFilter != "" || len(filter.IgnoreEndpoints) > 0 {
		result.AppliedFilter = &AppliedFilterEcho{
			URL:             filter.URLFilter,
			IgnoreEndpoints: filter.IgnoreEndpoints,
		}
	}

	return result
}

// buildEndpointReport creates a contract report for a single endpoint tracker.
func buildEndpointReport(endpoint string, tracker *EndpointTracker) EndpointContractReport {
	statusCodes := make(map[string]int)
	for _, status := range tracker.StatusHistory {
		statusCodes[fmt.Sprintf("%d", status)]++
	}

	consistency, score := computeConsistency(tracker.ConsistentCount, tracker.CallCount)
	parts := strings.SplitN(endpoint, " ", 2)

	report := EndpointContractReport{
		Endpoint:         endpoint,
		Method:           parts[0],
		CallCount:        tracker.CallCount,
		StatusCodes:      statusCodes,
		Consistency:      consistency,
		ConsistencyScore: score,
	}
	if !tracker.LastCalled.IsZero() {
		report.LastCalledAt = tracker.LastCalled.Format(time.RFC3339)
	}
	if !tracker.FirstCalled.IsZero() {
		report.FirstCalledAt = tracker.FirstCalled.Format(time.RFC3339)
	}
	if tracker.EstablishedShape != nil {
		if shapeMap, ok := tracker.EstablishedShape.(map[string]any); ok {
			report.EstablishedShape = toStringMap(shapeMap)
		}
	}
	return report
}

// computeConsistency returns the human-readable consistency string and numeric score.
func computeConsistency(consistentCount, callCount int) (string, float64) {
	if callCount == 0 {
		return "100%", 1.0
	}
	score := float64(consistentCount) / float64(callCount)
	return fmt.Sprintf("%.0f%%", score*100), score
}

// Clear resets all tracked endpoint data.
func (v *APIContractValidator) Clear() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.trackers = make(map[string]*EndpointTracker)
}

// GetTrackers returns a copy of trackers for testing.
func (v *APIContractValidator) GetTrackers() map[string]*EndpointTracker {
	v.mu.RLock()
	defer v.mu.RUnlock()

	copy := make(map[string]*EndpointTracker)
	for k, t := range v.trackers {
		copy[k] = t
	}
	return copy
}

// ============================================
// Helpers
// ============================================

func (v *APIContractValidator) matchesFilter(endpoint string, filter APIContractFilter) bool {
	if filter.URLFilter != "" && !strings.Contains(endpoint, filter.URLFilter) {
		return false
	}
	for _, ignore := range filter.IgnoreEndpoints {
		if strings.Contains(endpoint, ignore) {
			return false
		}
	}
	return true
}
