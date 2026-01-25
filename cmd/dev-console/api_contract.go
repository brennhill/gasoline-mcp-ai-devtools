// api_contract.go — API contract validation from observed network traffic.
// Tracks response shapes across requests, detects contract violations when
// shapes change unexpectedly, fields go missing, types change, or error
// responses replace success responses.
// Design: Learns schemas incrementally by merging new fields. Tracks field
// presence counts to distinguish required vs optional fields. Violations
// only flagged after minimum observations establish baseline shape.
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
	maxContractEndpoints     = 30  // Max tracked endpoints
	maxStatusHistory         = 20  // Status codes to keep per endpoint
	minCallsToEstablishShape = 3   // Observations needed before flagging violations
	maxViolationsPerEndpoint = 10  // Cap violations per endpoint
	maxShapeComparisonDepth  = 3   // Nested object comparison depth
)

// ============================================
// Types
// ============================================

// APIContractValidator tracks API response shapes and detects contract violations.
type APIContractValidator struct {
	mu       sync.RWMutex
	trackers map[string]*EndpointTracker
}

// EndpointTracker maintains the established shape for an endpoint.
type EndpointTracker struct {
	Endpoint         string                 `json:"endpoint"`          // "METHOD /path"
	EstablishedShape interface{}            `json:"established_shape"` // Learned response shape
	CallCount        int                    `json:"call_count"`
	SuccessCount     int                    `json:"success_count"`     // 2xx responses
	ConsistentCount  int                    `json:"consistent_count"`  // Responses matching established shape
	StatusHistory    []int                  `json:"status_history"`    // Last N status codes
	FieldPresence    map[string]int         `json:"field_presence"`    // field -> count of appearances
	FieldTypes       map[string]string      `json:"field_types"`       // field -> inferred type
	LastCalled       time.Time              `json:"last_called"`
	Violations       []APIContractViolation `json:"violations"`
}

// APIContractViolation represents a detected contract violation.
type APIContractViolation struct {
	Endpoint      string                 `json:"endpoint"`
	Type          string                 `json:"type"` // "shape_change", "type_change", "error_spike", "new_field", "null_field"
	Description   string                 `json:"description"`
	ExpectedShape map[string]interface{} `json:"expected_shape,omitempty"`
	ActualShape   map[string]interface{} `json:"actual_shape,omitempty"`
	MissingFields []string               `json:"missing_fields,omitempty"`
	NewFields     []string               `json:"new_fields,omitempty"`
	Field         string                 `json:"field,omitempty"`
	ExpectedType  string                 `json:"expected_type,omitempty"`
	ActualType    string                 `json:"actual_type,omitempty"`
	SampleValue   interface{}            `json:"sample_value,omitempty"`
	StatusHistory []int                  `json:"status_history,omitempty"`
	LastErrorBody map[string]interface{} `json:"last_error_body,omitempty"`
	Occurrences   *ViolationOccurrences  `json:"occurrences,omitempty"`
}

// ViolationOccurrences tracks when a violation was first seen and how often.
type ViolationOccurrences struct {
	ExpectedCount  int       `json:"expected_count"`
	ViolationCount int       `json:"violation_count"`
	FirstSeen      time.Time `json:"first_seen"`
	LastViolation  time.Time `json:"last_violation"`
}

// APIContractFilter controls which endpoints are analyzed.
type APIContractFilter struct {
	URLFilter       string   `json:"url_filter,omitempty"`
	IgnoreEndpoints []string `json:"ignore_endpoints,omitempty"`
}

// APIContractAnalyzeResult is the response from the analyze action.
type APIContractAnalyzeResult struct {
	Action                string                 `json:"action"`
	Violations            []APIContractViolation `json:"violations"`
	TrackedEndpoints      int                    `json:"tracked_endpoints"`
	TotalRequestsAnalyzed int                    `json:"total_requests_analyzed"`
	CleanEndpoints        int                    `json:"clean_endpoints"`
}

// APIContractReportResult is the response from the report action.
type APIContractReportResult struct {
	Action    string                   `json:"action"`
	Endpoints []EndpointContractReport `json:"endpoints"`
}

// EndpointContractReport summarizes a single endpoint's contract state.
type EndpointContractReport struct {
	Endpoint         string                 `json:"endpoint"`
	Method           string                 `json:"method"`
	CallCount        int                    `json:"call_count"`
	StatusCodes      map[string]int         `json:"status_codes"`
	EstablishedShape map[string]interface{} `json:"established_shape,omitempty"`
	Consistency      string                 `json:"consistency"`
	LastCalled       string                 `json:"last_called"`
}

// ============================================
// Constructor
// ============================================

// NewAPIContractValidator creates a new validator.
func NewAPIContractValidator() *APIContractValidator {
	return &APIContractValidator{
		trackers: make(map[string]*EndpointTracker),
	}
}

// ============================================
// Endpoint Normalization
// ============================================

var (
	contractUUIDPattern    = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	contractNumericPattern = regexp.MustCompile(`^\d+$`)
	contractHexPattern     = regexp.MustCompile(`^[0-9a-fA-F]{16,}$`)
)

// normalizeEndpoint converts a METHOD + URL into a normalized endpoint key.
// Dynamic segments (numeric IDs, UUIDs, hex hashes) are replaced with {id}.
// Query parameters are stripped.
func normalizeEndpoint(method, rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return method + " " + rawURL
	}

	path := parsed.Path
	if path == "" {
		path = "/"
	}

	// Normalize dynamic path segments
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if seg == "" {
			continue
		}
		// Replace UUIDs
		if contractUUIDPattern.MatchString(seg) {
			segments[i] = "{id}"
		} else if contractNumericPattern.MatchString(seg) {
			// Replace pure numeric IDs
			segments[i] = "{id}"
		} else if contractHexPattern.MatchString(seg) {
			// Replace long hex strings
			segments[i] = "{id}"
		}
	}

	normalizedPath := strings.Join(segments, "/")
	return method + " " + normalizedPath
}

// ============================================
// Learning
// ============================================

// Learn records a network body observation for contract tracking.
func (v *APIContractValidator) Learn(body NetworkBody) {
	v.mu.Lock()
	defer v.mu.Unlock()

	endpoint := normalizeEndpoint(body.Method, body.URL)

	// Check endpoint limit
	if _, exists := v.trackers[endpoint]; !exists {
		if len(v.trackers) >= maxContractEndpoints {
			return
		}
	}

	tracker := v.getOrCreateTracker(endpoint)
	tracker.CallCount++
	tracker.LastCalled = time.Now()

	// Track status code
	tracker.StatusHistory = append(tracker.StatusHistory, body.Status)
	if len(tracker.StatusHistory) > maxStatusHistory {
		tracker.StatusHistory = tracker.StatusHistory[1:]
	}

	// Only learn shape from successful responses
	if body.Status >= 200 && body.Status < 300 {
		tracker.SuccessCount++
		v.learnShape(tracker, body)
	}
}

func (v *APIContractValidator) getOrCreateTracker(endpoint string) *EndpointTracker {
	tracker, exists := v.trackers[endpoint]
	if !exists {
		tracker = &EndpointTracker{
			Endpoint:      endpoint,
			FieldPresence: make(map[string]int),
			FieldTypes:    make(map[string]string),
			StatusHistory: make([]int, 0),
			Violations:    make([]APIContractViolation, 0),
		}
		v.trackers[endpoint] = tracker
	}
	return tracker
}

func (v *APIContractValidator) learnShape(tracker *EndpointTracker, body NetworkBody) {
	// Skip non-JSON or empty responses
	if body.ResponseBody == "" {
		return
	}
	if body.ContentType != "" && !strings.Contains(body.ContentType, "json") {
		return
	}

	var parsed interface{}
	if err := json.Unmarshal([]byte(body.ResponseBody), &parsed); err != nil {
		return
	}

	// Extract shape from parsed JSON
	shape := v.extractShape(parsed, 0)

	// Merge into established shape
	if tracker.EstablishedShape == nil {
		tracker.EstablishedShape = shape
		tracker.ConsistentCount++ // First response is by definition consistent
	} else {
		// Check if this response is consistent with established shape before merging
		shapeViolations := v.compareShapes(tracker.Endpoint, tracker.EstablishedShape, shape, parsed)
		if len(shapeViolations) == 0 {
			tracker.ConsistentCount++
		}
		tracker.EstablishedShape = v.mergeShapes(tracker.EstablishedShape, shape)
	}

	// Track field presence for objects
	if objShape, ok := shape.(map[string]interface{}); ok {
		for field, fieldType := range objShape {
			tracker.FieldPresence[field]++
			if typeStr, ok := fieldType.(string); ok {
				tracker.FieldTypes[field] = typeStr
			}
		}
	}
}

// extractShape extracts a type-only shape from a JSON value.
func (v *APIContractValidator) extractShape(value interface{}, depth int) interface{} {
	if depth > maxShapeComparisonDepth {
		return "object" // Truncate deep nesting
	}

	switch val := value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []interface{}:
		if len(val) > 0 {
			// Use first element to infer array element type
			elemShape := v.extractShape(val[0], depth+1)
			return map[string]interface{}{"$array": elemShape}
		}
		return "array"
	case map[string]interface{}:
		shape := make(map[string]interface{})
		for k, fieldVal := range val {
			shape[k] = v.extractShape(fieldVal, depth+1)
		}
		return shape
	default:
		return "unknown"
	}
}

// mergeShapes combines two shapes, keeping all observed fields.
func (v *APIContractValidator) mergeShapes(existing, incoming interface{}) interface{} {
	existingMap, eOK := existing.(map[string]interface{})
	incomingMap, iOK := incoming.(map[string]interface{})

	if !eOK || !iOK {
		// If types differ, prefer the existing (already established)
		return existing
	}

	// Merge all fields from both
	merged := make(map[string]interface{})
	for k, val := range existingMap {
		merged[k] = val
	}
	for k, val := range incomingMap {
		if existingVal, exists := merged[k]; !exists {
			merged[k] = val
		} else {
			// Recursively merge nested objects
			merged[k] = v.mergeShapes(existingVal, val)
		}
	}
	return merged
}

// ============================================
// Validation
// ============================================

// Validate checks a network body against the learned schema and returns violations.
func (v *APIContractValidator) Validate(body NetworkBody) []APIContractViolation {
	v.mu.Lock()
	defer v.mu.Unlock()

	endpoint := normalizeEndpoint(body.Method, body.URL)
	tracker := v.getOrCreateTracker(endpoint)
	tracker.CallCount++
	tracker.LastCalled = time.Now()

	// Track status
	tracker.StatusHistory = append(tracker.StatusHistory, body.Status)
	if len(tracker.StatusHistory) > maxStatusHistory {
		tracker.StatusHistory = tracker.StatusHistory[1:]
	}

	var violations []APIContractViolation

	// Check for error spike
	if body.Status >= 400 {
		spike := v.detectErrorSpike(tracker, body)
		if spike != nil {
			violations = append(violations, *spike)
			v.addViolation(tracker, *spike)
		}
		return violations // Don't validate shape for error responses
	}

	// Skip validation if shape not yet established
	if tracker.SuccessCount < minCallsToEstablishShape {
		tracker.SuccessCount++
		v.learnShape(tracker, body)
		return violations
	}

	// Parse response
	if body.ResponseBody == "" || (body.ContentType != "" && !strings.Contains(body.ContentType, "json")) {
		return violations
	}

	var parsed interface{}
	if err := json.Unmarshal([]byte(body.ResponseBody), &parsed); err != nil {
		return violations
	}

	actualShape := v.extractShape(parsed, 0)

	// Compare shapes
	shapeViolations := v.compareShapes(endpoint, tracker.EstablishedShape, actualShape, parsed)
	for _, viol := range shapeViolations {
		violations = append(violations, viol)
		v.addViolation(tracker, viol)
	}

	// Update consistency tracking
	if len(shapeViolations) == 0 {
		tracker.ConsistentCount++
	}

	return violations
}

// detectErrorSpike checks if there's a pattern of success followed by errors.
func (v *APIContractValidator) detectErrorSpike(tracker *EndpointTracker, body NetworkBody) *APIContractViolation {
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
		var errorBody map[string]interface{}
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
func (v *APIContractValidator) compareShapes(endpoint string, expected, actual, actualData interface{}) []APIContractViolation {
	var violations []APIContractViolation

	expectedMap, eOK := expected.(map[string]interface{})
	actualMap, aOK := actual.(map[string]interface{})

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
	actualDataMap, _ := actualData.(map[string]interface{})
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
			var sampleValue interface{}
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
	if len(tracker.Violations) >= maxViolationsPerEndpoint {
		// Remove oldest
		tracker.Violations = tracker.Violations[1:]
	}
	tracker.Violations = append(tracker.Violations, violation)
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

	for endpoint, tracker := range v.trackers {
		if !v.matchesFilter(endpoint, filter) {
			continue
		}
		trackedEndpoints++
		totalRequests += tracker.CallCount

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

	return APIContractAnalyzeResult{
		Action:                "analyzed",
		Violations:            violations,
		TrackedEndpoints:      trackedEndpoints,
		TotalRequestsAnalyzed: totalRequests,
		CleanEndpoints:        cleanEndpoints,
	}
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

		// Extract method from endpoint
		parts := strings.SplitN(endpoint, " ", 2)
		method := parts[0]

		report := EndpointContractReport{
			Endpoint:    endpoint,
			Method:      method,
			CallCount:   tracker.CallCount,
			StatusCodes: statusCodes,
			Consistency: consistency,
		}

		if !tracker.LastCalled.IsZero() {
			report.LastCalled = tracker.LastCalled.Format(time.RFC3339)
		}

		if tracker.EstablishedShape != nil {
			if shapeMap, ok := tracker.EstablishedShape.(map[string]interface{}); ok {
				report.EstablishedShape = toStringMap(shapeMap)
			}
		}

		endpoints = append(endpoints, report)
	}

	// Sort by call count (most used first)
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].CallCount > endpoints[j].CallCount
	})

	return APIContractReportResult{
		Action:    "report",
		Endpoints: endpoints,
	}
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

func describeType(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		return v // Already a type string
	case map[string]interface{}:
		if _, hasArray := v["$array"]; hasArray {
			return "array"
		}
		return "object"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func toStringMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		switch val := v.(type) {
		case string:
			result[k] = val
		case map[string]interface{}:
			result[k] = toStringMap(val)
		default:
			result[k] = describeType(v)
		}
	}
	return result
}
