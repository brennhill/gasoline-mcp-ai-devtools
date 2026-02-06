// api_contract.go — API contract validation from observed network traffic.
// Tracks response shapes across requests, detects contract violations when
// shapes change unexpectedly, fields go missing, types change, or error
// responses replace success responses.
// Design: Learns schemas incrementally by merging new fields. Tracks field
// presence counts to distinguish required vs optional fields. Violations
// only flagged after minimum observations establish baseline shape.
package analysis

import (
	"encoding/json"
	"fmt"
	"github.com/dev-console/dev-console/internal/capture"
	"net/url"
	"regexp"
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
	EstablishedShape any                    `json:"established_shape"` // Learned response shape
	CallCount        int                    `json:"call_count"`
	SuccessCount     int                    `json:"success_count"`     // 2xx responses
	ConsistentCount  int                    `json:"consistent_count"`  // Responses matching established shape
	StatusHistory    []int                  `json:"status_history"`    // Last N status codes
	FieldPresence    map[string]int         `json:"field_presence"`    // field -> count of appearances
	FieldTypes       map[string]string      `json:"field_types"`       // field -> inferred type
	FirstCalled      time.Time              `json:"first_called"`      // When endpoint was first seen
	LastCalled       time.Time              `json:"last_called"`
	Violations       []APIContractViolation `json:"violations"`
}

// APIContractViolation represents a detected contract violation.
type APIContractViolation struct {
	Endpoint          string                 `json:"endpoint"`
	Type              string                 `json:"type"`           // "shape_change", "type_change", "error_spike", "new_field", "null_field"
	ViolationType     string                 `json:"violation_type"` // Same as Type, explicit for LLM consumption
	Severity          string                 `json:"severity"`       // "critical", "high", "medium", "low"
	Description       string                 `json:"description"`
	AffectedCallCount int                    `json:"affected_call_count"`        // How many calls violated this rule
	FirstSeenAt       string                 `json:"first_seen_at,omitempty"`    // RFC3339 when violation first detected
	LastSeenAt        string                 `json:"last_seen_at,omitempty"`     // RFC3339 when violation last detected
	ExpectedShape     map[string]any         `json:"expected_shape,omitempty"`
	ActualShape       map[string]any         `json:"actual_shape,omitempty"`
	MissingFields     []string               `json:"missing_fields,omitempty"`
	NewFields         []string               `json:"new_fields,omitempty"`
	Field             string                 `json:"field,omitempty"`
	ExpectedType      string                 `json:"expected_type,omitempty"`
	ActualType        string                 `json:"actual_type,omitempty"`
	SampleValue       any                    `json:"sample_value,omitempty"`
	StatusHistory     []int                  `json:"status_history,omitempty"`
	LastErrorBody     map[string]any         `json:"last_error_body,omitempty"`
	Occurrences       *ViolationOccurrences  `json:"occurrences,omitempty"`
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
	URLFilter       string   `json:"url,omitempty"`
	IgnoreEndpoints []string `json:"ignore_endpoints,omitempty"`
}

// AnalyzeSummary provides aggregate counts for LLM consumption.
type AnalyzeSummary struct {
	Violations     int `json:"violations"`
	Endpoints      int `json:"endpoints"`
	TotalRequests  int `json:"total_requests"`
	CleanEndpoints int `json:"clean_endpoints"`
}

// AppliedFilterEcho echoes back the filter parameters that were applied.
type AppliedFilterEcho struct {
	URL             string   `json:"url,omitempty"`
	IgnoreEndpoints []string `json:"ignore_endpoints,omitempty"`
}

// APIContractAnalyzeResult is the response from the analyze action.
type APIContractAnalyzeResult struct {
	Action                 string                 `json:"action"`
	AnalyzedAt             string                 `json:"analyzed_at"`
	DataWindowStartedAt    string                 `json:"data_window_started_at,omitempty"`
	AppliedFilter          *AppliedFilterEcho     `json:"applied_filter,omitempty"`
	Summary                *AnalyzeSummary        `json:"summary"`                           // Aggregate counts
	Violations             []APIContractViolation `json:"violations"`
	TrackedEndpoints       int                    `json:"tracked_endpoints"`
	TotalRequestsAnalyzed  int                    `json:"total_requests_analyzed"`
	CleanEndpoints         int                    `json:"clean_endpoints"`
	PossibleViolationTypes []string               `json:"possible_violation_types"`
	Hint                   string                 `json:"hint,omitempty"`         // Helpful hint when no violations found
}

// APIContractReportResult is the response from the report action.
type APIContractReportResult struct {
	Action            string                   `json:"action"`
	AnalyzedAt        string                   `json:"analyzed_at"`
	AppliedFilter     *AppliedFilterEcho       `json:"applied_filter,omitempty"`
	Endpoints         []EndpointContractReport `json:"endpoints"`
	ConsistencyLevels map[string]string        `json:"consistency_levels"`
}

// EndpointContractReport summarizes a single endpoint's contract state.
type EndpointContractReport struct {
	Endpoint         string         `json:"endpoint"`
	Method           string         `json:"method"`
	CallCount        int            `json:"call_count"`
	StatusCodes      map[string]int `json:"status_codes"`
	EstablishedShape map[string]any `json:"established_shape,omitempty"`
	Consistency      string         `json:"consistency"`
	ConsistencyScore float64        `json:"consistency_score"` // Numeric 0-1
	FirstCalledAt    string         `json:"first_called_at"`
	LastCalledAt     string         `json:"last_called_at"`
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
func (v *APIContractValidator) Learn(body capture.NetworkBody) {
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
	now := time.Now()
	if tracker.FirstCalled.IsZero() {
		tracker.FirstCalled = now
	}
	tracker.LastCalled = now

	// Track status code
	tracker.StatusHistory = append(tracker.StatusHistory, body.Status)
	if len(tracker.StatusHistory) > maxStatusHistory {
		newHistory := make([]int, len(tracker.StatusHistory)-1)
		copy(newHistory, tracker.StatusHistory[1:])
		tracker.StatusHistory = newHistory
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

func (v *APIContractValidator) learnShape(tracker *EndpointTracker, body capture.NetworkBody) {
	// Skip non-JSON or empty responses
	if body.ResponseBody == "" {
		return
	}
	if body.ContentType != "" && !strings.Contains(body.ContentType, "json") {
		return
	}

	var parsed any
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
	if objShape, ok := shape.(map[string]any); ok {
		for field, fieldType := range objShape {
			tracker.FieldPresence[field]++
			if typeStr, ok := fieldType.(string); ok {
				tracker.FieldTypes[field] = typeStr
			}
		}
	}
}

// extractShape extracts a type-only shape from a JSON value.
func (v *APIContractValidator) extractShape(value any, depth int) any {
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
	case []any:
		if len(val) > 0 {
			// Use first element to infer array element type
			elemShape := v.extractShape(val[0], depth+1)
			return map[string]any{"$array": elemShape}
		}
		return "array"
	case map[string]any:
		shape := make(map[string]any)
		for k, fieldVal := range val {
			shape[k] = v.extractShape(fieldVal, depth+1)
		}
		return shape
	default:
		return "unknown"
	}
}

// mergeShapes combines two shapes, keeping all observed fields.
func (v *APIContractValidator) mergeShapes(existing, incoming any) any {
	existingMap, eOK := existing.(map[string]any)
	incomingMap, iOK := incoming.(map[string]any)

	if !eOK || !iOK {
		// If types differ, prefer the existing (already established)
		return existing
	}

	// Merge all fields from both
	merged := make(map[string]any)
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
func (v *APIContractValidator) Validate(body capture.NetworkBody) []APIContractViolation {
	v.mu.Lock()
	defer v.mu.Unlock()

	endpoint := normalizeEndpoint(body.Method, body.URL)
	tracker := v.getOrCreateTracker(endpoint)
	tracker.CallCount++
	now := time.Now()
	if tracker.FirstCalled.IsZero() {
		tracker.FirstCalled = now
	}
	tracker.LastCalled = now

	// Track status
	tracker.StatusHistory = append(tracker.StatusHistory, body.Status)
	if len(tracker.StatusHistory) > maxStatusHistory {
		newHistory := make([]int, len(tracker.StatusHistory)-1)
		copy(newHistory, tracker.StatusHistory[1:])
		tracker.StatusHistory = newHistory
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

	var parsed any
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

// ============================================
// Helpers
// ============================================

func describeType(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		return v // Already a type string
	case map[string]any:
		if _, hasArray := v["$array"]; hasArray {
			return "array"
		}
		return "object"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func toStringMap(m map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		switch val := v.(type) {
		case string:
			result[k] = val
		case map[string]any:
			result[k] = toStringMap(val)
		default:
			result[k] = describeType(v)
		}
	}
	return result
}
