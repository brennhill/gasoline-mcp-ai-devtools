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
	var violations []APIContractViolation

	expectedMap, eOK := expected.(map[string]any)
	actualMap, aOK := actual.(map[string]any)

	if !eOK || !aOK {
		// Top-level type change
		if fmt.Sprintf("%T", expected) != fmt.Sprintf("%T", actual) {
			violations = append(violations, APIContractViolation{
				Endpoint:     endpoint,
				Type:         "type_change",
				Description:  "Response type changed",
				ExpectedType: describeType(expected),
				ActualType:   describeType(actual),
			})
		}
		return violations
	}

	// Check for missing fields
	var missingFields []string
	for field := range expectedMap {
		if _, found := actualMap[field]; !found {
			// Skip $array marker
			if field == "$array" {
				continue
			}
			missingFields = append(missingFields, field)
		}
	}
	if len(missingFields) > 0 {
		sort.Strings(missingFields)
		violations = append(violations, APIContractViolation{
			Endpoint:      endpoint,
			Type:          "shape_change",
			Description:   fmt.Sprintf("Field(s) missing from response: %s", strings.Join(missingFields, ", ")),
			MissingFields: missingFields,
			ExpectedShape: toStringMap(expectedMap),
			ActualShape:   toStringMap(actualMap),
		})
	}

	// Check for new fields
	var newFields []string
	for field := range actualMap {
		if _, found := expectedMap[field]; !found {
			if field == "$array" {
				continue
			}
			newFields = append(newFields, field)
		}
	}
	if len(newFields) > 0 {
		sort.Strings(newFields)
		violations = append(violations, APIContractViolation{
			Endpoint:    endpoint,
			Type:        "new_field",
			Description: fmt.Sprintf("New field(s) appeared in response: %s", strings.Join(newFields, ", ")),
			NewFields:   newFields,
		})
	}

	// Check for type changes in existing fields
	actualDataMap, _ := actualData.(map[string]any)
	for field, expectedType := range expectedMap {
		actualType, found := actualMap[field]
		if !found {
			continue
		}

		expectedTypeStr := describeType(expectedType)
		actualTypeStr := describeType(actualType)

		// Check for null transition
		if actualTypeStr == "null" && expectedTypeStr != "null" {
			violations = append(violations, APIContractViolation{
				Endpoint:     endpoint,
				Type:         "null_field",
				Description:  fmt.Sprintf("Field '%s' became null (was %s)", field, expectedTypeStr),
				Field:        field,
				ExpectedType: expectedTypeStr,
				ActualType:   "null",
			})
		} else if expectedTypeStr != actualTypeStr && actualTypeStr != "null" {
			var sampleValue any
			if actualDataMap != nil {
				sampleValue = actualDataMap[field]
			}
			violations = append(violations, APIContractViolation{
				Endpoint:     endpoint,
				Type:         "type_change",
				Description:  fmt.Sprintf("Field '%s' changed type from %s to %s", field, expectedTypeStr, actualTypeStr),
				Field:        field,
				ExpectedType: expectedTypeStr,
				ActualType:   actualTypeStr,
				SampleValue:  sampleValue,
			})
		}
	}

	return violations
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

// Analyze processes tracked endpoints and returns all violations.
func (v *APIContractValidator) Analyze(filter APIContractFilter) APIContractAnalyzeResult {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var violations []APIContractViolation
	totalRequests := 0
	cleanEndpoints := 0
	trackedEndpoints := 0
	var earliestCall time.Time

	for endpoint, tracker := range v.trackers {
		if !v.matchesFilter(endpoint, filter) {
			continue
		}
		trackedEndpoints++
		totalRequests += tracker.CallCount

		// Track earliest call for dataWindowStartedAt
		if !tracker.FirstCalled.IsZero() && (earliestCall.IsZero() || tracker.FirstCalled.Before(earliestCall)) {
			earliestCall = tracker.FirstCalled
		}

		if len(tracker.Violations) > 0 {
			violations = append(violations, tracker.Violations...)
		} else {
			cleanEndpoints++
		}
	}

	// Sort violations by endpoint for deterministic output
	sort.Slice(violations, func(i, j int) bool {
		return violations[i].Endpoint < violations[j].Endpoint
	})

	result := APIContractAnalyzeResult{
		Action:                "analyzed",
		AnalyzedAt:            time.Now().Format(time.RFC3339),
		Violations:            violations,
		TrackedEndpoints:      trackedEndpoints,
		TotalRequestsAnalyzed: totalRequests,
		CleanEndpoints:        cleanEndpoints,
		Summary: &AnalyzeSummary{
			Violations:     len(violations),
			Endpoints:      trackedEndpoints,
			TotalRequests:  totalRequests,
			CleanEndpoints: cleanEndpoints,
		},
		PossibleViolationTypes: []string{"shape_change", "type_change", "error_spike", "new_field", "null_field"},
	}

	// dataWindowStartedAt: when data collection began
	if !earliestCall.IsZero() {
		result.DataWindowStartedAt = earliestCall.Format(time.RFC3339)
	}

	// appliedFilter echo
	if filter.URLFilter != "" || len(filter.IgnoreEndpoints) > 0 {
		result.AppliedFilter = &AppliedFilterEcho{
			URL:             filter.URLFilter,
			IgnoreEndpoints: filter.IgnoreEndpoints,
		}
	}

	// Helpful hint when no violations found
	if len(violations) == 0 {
		if trackedEndpoints > 0 {
			result.Hint = fmt.Sprintf("No violations detected. All %d tracked endpoint(s) have consistent response shapes.", trackedEndpoints)
		} else {
			result.Hint = "No violations detected. No endpoints tracked yet — browse your application to capture API traffic."
		}
	}

	return result
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

		// Build status code map
		statusCodes := make(map[string]int)
		for _, status := range tracker.StatusHistory {
			key := fmt.Sprintf("%d", status)
			statusCodes[key]++
		}

		// Calculate consistency percentage
		consistency := "100%"
		if tracker.CallCount > 0 {
			pct := float64(tracker.ConsistentCount) / float64(tracker.CallCount) * 100
			consistency = fmt.Sprintf("%.0f%%", pct)
		}

		// Calculate consistency score (0-1)
		consistencyScore := 1.0
		if tracker.CallCount > 0 {
			consistencyScore = float64(tracker.ConsistentCount) / float64(tracker.CallCount)
		}

		// Extract method from endpoint
		parts := strings.SplitN(endpoint, " ", 2)
		method := parts[0]

		report := EndpointContractReport{
			Endpoint:         endpoint,
			Method:           method,
			CallCount:        tracker.CallCount,
			StatusCodes:      statusCodes,
			Consistency:      consistency,
			ConsistencyScore: consistencyScore,
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

		endpoints = append(endpoints, report)
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
