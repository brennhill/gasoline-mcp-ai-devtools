// api_schema.go — API schema inference from observed network traffic.
// Builds endpoint patterns by normalizing dynamic path segments, tracking
// HTTP methods, status codes, and inferring response shapes from JSON bodies.
// Design: Path normalization detects UUID/numeric segments and replaces with
// :id placeholders. Response shapes use recursive type inference (string,
// number, bool, array, object). Output in gasoline or OpenAPI stub format.
package analysis

import (
	"encoding/json"
	"github.com/dev-console/dev-console/internal/capture"
	"math"
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
	maxSchemaEndpoints  = 200
	maxLatencySamples   = 100
	maxActualPaths      = 20
	maxQueryParamValues = 10
	maxResponseShapes   = 10
	maxWSSchemaConns    = 50
	maxWSMessageTypes   = 20
)

// ============================================
// Types
// ============================================

// SchemaFilter controls what endpoints are returned
type SchemaFilter struct {
	URLFilter       string
	MinObservations int
}

// APISchema is the top-level response from get_api_schema
type APISchema struct {
	Endpoints   []EndpointSchema `json:"endpoints"`
	WebSockets  []WSSchema       `json:"websockets,omitempty"`
	AuthPattern *AuthPattern     `json:"auth_pattern,omitempty"`
	Coverage    CoverageStats    `json:"coverage"`
}

// EndpointSchema describes one inferred API endpoint
type EndpointSchema struct {
	Method           string             `json:"method"`
	PathPattern      string             `json:"path_pattern"`
	LastPath         string             `json:"last_path,omitempty"`
	ObservationCount int                `json:"observation_count"`
	LastSeen         string             `json:"last_seen,omitempty"`
	PathParams       []PathParam        `json:"path_params,omitempty"`
	QueryParams      []QueryParam       `json:"query_params,omitempty"`
	RequestShape     *BodyShape         `json:"request_shape,omitempty"`
	ResponseShapes   map[int]*BodyShape `json:"response_shapes,omitempty"`
	Timing           TimingStats        `json:"timing"`
}

// PathParam describes a detected path parameter
type PathParam struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Position int    `json:"position"`
}

// QueryParam describes a detected query parameter
type QueryParam struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	Required       bool     `json:"required"`
	ObservedValues []string `json:"observed_values,omitempty"`
}

// BodyShape describes the inferred shape of a request or response body
type BodyShape struct {
	ContentType string                 `json:"content_type,omitempty"`
	Fields      map[string]FieldSchema `json:"fields,omitempty"`
	Count       int                    `json:"count"`
}

// FieldSchema describes a single field in a body shape
type FieldSchema struct {
	Type     string `json:"type"`
	Format   string `json:"format,omitempty"`
	Required bool   `json:"required"`
	Observed int    `json:"observed"`
}

// TimingStats holds computed latency statistics
type TimingStats struct {
	Avg float64 `json:"avg"`
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	Max float64 `json:"max"`
}

// AuthPattern describes detected authentication patterns
type AuthPattern struct {
	Type        string   `json:"type"`
	Header      string   `json:"header"`
	AuthRate    float64  `json:"auth_rate_percent"`
	PublicPaths []string `json:"public_paths,omitempty"`
}

// WSSchema describes inferred WebSocket message patterns
type WSSchema struct {
	URL           string   `json:"url"`
	TotalMessages int      `json:"total_messages"`
	MessageTypes  []string `json:"message_types,omitempty"`
	IncomingCount int      `json:"incoming_count"`
	OutgoingCount int      `json:"outgoing_count"`
}

// CoverageStats holds API coverage statistics
type CoverageStats struct {
	TotalEndpoints int            `json:"total_endpoints"`
	Methods        map[string]int `json:"methods"`
	ErrorRate      float64        `json:"error_rate_percent"`
	AvgResponseMs  float64        `json:"avg_response_ms"`
}

// ============================================
// Internal accumulator types
// ============================================

// endpointAccumulator collects observations for one METHOD+path_pattern
type endpointAccumulator struct {
	method           string
	pathPattern      string
	lastPath         string
	observationCount int
	lastSeen         time.Time
	actualPaths      []string

	// Query params: name -> paramAccumulator
	queryParams map[string]*paramAccumulator

	// Request body field tracking
	requestFields map[string]*fieldAccumulator
	requestCount  int

	// Response shapes: statusCode -> responseAccumulator
	responseShapes map[int]*responseAccumulator

	// Latency samples
	latencies []float64

	// Total observations for required field calculation
	totalObservations int

	// Track status codes for error rate
	errorCount int
}

// paramAccumulator tracks query parameter values and occurrences
type paramAccumulator struct {
	count      int
	values     []string
	allNumeric bool
	allBoolean bool
}

// fieldAccumulator tracks a body field across observations
type fieldAccumulator struct {
	typeCounts map[string]int // type -> count
	format     string
	observed   int
}

// responseAccumulator tracks response shape for a specific status code
type responseAccumulator struct {
	count       int
	contentType string
	fields      map[string]*fieldAccumulator
}

// wsAccumulator tracks WebSocket message patterns for a connection URL
type wsAccumulator struct {
	url           string
	totalMessages int
	incomingCount int
	outgoingCount int
	messageTypes  map[string]bool
}

// ============================================
// SchemaStore
// ============================================

// SchemaStore manages API schema inference from observed network traffic
type SchemaStore struct {
	mu           sync.RWMutex
	accumulators map[string]*endpointAccumulator // key: "METHOD /path/pattern"
	wsSchemas    map[string]*wsAccumulator       // key: URL
}

// NewSchemaStore creates a new SchemaStore
func NewSchemaStore() *SchemaStore {
	return &SchemaStore{
		accumulators: make(map[string]*endpointAccumulator),
		wsSchemas:    make(map[string]*wsAccumulator),
	}
}

// EndpointCount returns the number of observed API endpoints.
func (s *SchemaStore) EndpointCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.accumulators)
}

// ============================================
// Path Parameterization
// ============================================

var (
	uuidPattern    = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	numericPattern = regexp.MustCompile(`^\d+$`)
	hexHashPattern = regexp.MustCompile(`^[0-9a-fA-F]{16,}$`)
)

// parameterizePath replaces dynamic path segments with placeholders
func parameterizePath(path string) string {
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if seg == "" {
			continue
		}
		if uuidPattern.MatchString(seg) {
			segments[i] = "{uuid}"
		} else if numericPattern.MatchString(seg) {
			segments[i] = "{id}"
		} else if hexHashPattern.MatchString(seg) {
			segments[i] = "{hash}"
		}
	}
	return strings.Join(segments, "/")
}

// ============================================
// Observation
// ============================================

// Observe records a network body observation for schema inference
func (s *SchemaStore) Observe(body capture.NetworkBody) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Parse URL to get path and query params
	parsedURL, err := url.Parse(body.URL)
	if err != nil {
		return
	}

	path := parsedURL.Path
	if path == "" {
		path = "/"
	}

	pattern := parameterizePath(path)
	key := body.Method + " " + pattern

	// Check endpoint cap
	if _, exists := s.accumulators[key]; !exists {
		if len(s.accumulators) >= maxSchemaEndpoints {
			return
		}
	}

	acc := s.getOrCreateAccumulator(key, body.Method, pattern)
	acc.observationCount++
	acc.totalObservations++
	acc.lastSeen = time.Now()
	acc.lastPath = path

	// Track actual paths (up to max)
	if len(acc.actualPaths) < maxActualPaths {
		acc.actualPaths = append(acc.actualPaths, path)
	}

	// Track query parameters
	s.observeQueryParams(acc, parsedURL.Query())

	// Track latency
	if body.Duration > 0 && len(acc.latencies) < maxLatencySamples {
		acc.latencies = append(acc.latencies, float64(body.Duration))
	}

	// Track error count
	if body.Status >= 400 {
		acc.errorCount++
	}

	// Parse request body for shape inference (JSON only)
	if body.RequestBody != "" && isJSONContentType(body.ContentType) {
		s.observeRequestBody(acc, body.RequestBody)
	}

	// Parse response body for shape inference (JSON only)
	if body.ResponseBody != "" && isJSONContentType(body.ContentType) {
		s.observeResponseBody(acc, body.Status, body.ResponseBody, body.ContentType)
	} else if body.Status > 0 {
		// Still track the status code even if no body to parse
		if acc.responseShapes == nil {
			acc.responseShapes = make(map[int]*responseAccumulator)
		}
		if _, exists := acc.responseShapes[body.Status]; !exists {
			if len(acc.responseShapes) < maxResponseShapes {
				acc.responseShapes[body.Status] = &responseAccumulator{
					count:  1,
					fields: make(map[string]*fieldAccumulator),
				}
			}
		} else {
			acc.responseShapes[body.Status].count++
		}
	}
}

// ObserveWebSocket records a WebSocket event for schema inference
func (s *SchemaStore) ObserveWebSocket(event capture.WebSocketEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wsURL := event.URL
	if wsURL == "" {
		return
	}

	if len(s.wsSchemas) >= maxWSSchemaConns {
		if _, exists := s.wsSchemas[wsURL]; !exists {
			return
		}
	}

	ws, exists := s.wsSchemas[wsURL]
	if !exists {
		ws = &wsAccumulator{
			url:          wsURL,
			messageTypes: make(map[string]bool),
		}
		s.wsSchemas[wsURL] = ws
	}

	ws.totalMessages++
	switch event.Direction {
	case "incoming":
		ws.incomingCount++
	case "outgoing":
		ws.outgoingCount++
	}

	// Try to detect "type" or "action" field in JSON messages
	if event.Data != "" {
		var msg map[string]any
		if json.Unmarshal([]byte(event.Data), &msg) == nil {
			if typeVal, ok := msg["type"]; ok {
				if typeStr, ok := typeVal.(string); ok && len(ws.messageTypes) < maxWSMessageTypes {
					ws.messageTypes[typeStr] = true
				}
			}
			if actionVal, ok := msg["action"]; ok {
				if actionStr, ok := actionVal.(string); ok && len(ws.messageTypes) < maxWSMessageTypes {
					ws.messageTypes[actionStr] = true
				}
			}
		}
	}
}

func (s *SchemaStore) getOrCreateAccumulator(key, method, pattern string) *endpointAccumulator {
	acc, exists := s.accumulators[key]
	if !exists {
		acc = &endpointAccumulator{
			method:         method,
			pathPattern:    pattern,
			queryParams:    make(map[string]*paramAccumulator),
			requestFields:  make(map[string]*fieldAccumulator),
			responseShapes: make(map[int]*responseAccumulator),
			actualPaths:    make([]string, 0),
			latencies:      make([]float64, 0),
		}
		s.accumulators[key] = acc
	}
	return acc
}

func (s *SchemaStore) observeQueryParams(acc *endpointAccumulator, params url.Values) {
	for name, values := range params {
		pa, exists := acc.queryParams[name]
		if !exists {
			pa = &paramAccumulator{
				values:     make([]string, 0),
				allNumeric: true,
				allBoolean: true,
			}
			acc.queryParams[name] = pa
		}
		pa.count++

		for _, v := range values {
			// Track unique values up to max
			if len(pa.values) < maxQueryParamValues {
				found := false
				for _, existing := range pa.values {
					if existing == v {
						found = true
						break
					}
				}
				if !found {
					pa.values = append(pa.values, v)
				}
			}

			// Type inference
			if !numericPattern.MatchString(v) {
				pa.allNumeric = false
			}
			if v != "true" && v != "false" {
				pa.allBoolean = false
			}
		}
	}
}

func (s *SchemaStore) observeRequestBody(acc *endpointAccumulator, body string) {
	var parsed map[string]any
	if json.Unmarshal([]byte(body), &parsed) != nil {
		return
	}

	acc.requestCount++
	for field, value := range parsed {
		fa, exists := acc.requestFields[field]
		if !exists {
			fa = &fieldAccumulator{
				typeCounts: make(map[string]int),
			}
			acc.requestFields[field] = fa
		}
		fa.observed++
		fieldType, format := inferTypeAndFormat(value)
		fa.typeCounts[fieldType]++
		if format != "" {
			fa.format = format
		}
	}
}

func (s *SchemaStore) observeResponseBody(acc *endpointAccumulator, status int, body, contentType string) {
	var parsed map[string]any
	if json.Unmarshal([]byte(body), &parsed) != nil {
		return
	}

	if acc.responseShapes == nil {
		acc.responseShapes = make(map[int]*responseAccumulator)
	}

	ra, exists := acc.responseShapes[status]
	if !exists {
		if len(acc.responseShapes) >= maxResponseShapes {
			return
		}
		ra = &responseAccumulator{
			contentType: contentType,
			fields:      make(map[string]*fieldAccumulator),
		}
		acc.responseShapes[status] = ra
	}
	ra.count++
	ra.contentType = contentType

	for field, value := range parsed {
		fa, exists := ra.fields[field]
		if !exists {
			fa = &fieldAccumulator{
				typeCounts: make(map[string]int),
			}
			ra.fields[field] = fa
		}
		fa.observed++
		fieldType, format := inferTypeAndFormat(value)
		fa.typeCounts[fieldType]++
		if format != "" {
			fa.format = format
		}
	}
}

// ============================================
// Type and Format Inference
// ============================================

var (
	uuidValuePattern     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	datetimeValuePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`)
)

func inferTypeAndFormat(value any) (string, string) {
	switch v := value.(type) {
	case nil:
		return "null", ""
	case bool:
		return "boolean", ""
	case float64:
		if v == math.Trunc(v) {
			return "integer", ""
		}
		return "number", ""
	case string:
		format := inferStringFormat(v)
		return "string", format
	case []any:
		return "array", ""
	case map[string]any:
		return "object", ""
	default:
		return "string", ""
	}
}

func inferStringFormat(v string) string {
	if uuidValuePattern.MatchString(v) {
		return "uuid"
	}
	if datetimeValuePattern.MatchString(v) {
		return "datetime"
	}
	if strings.Contains(v, "@") && strings.Contains(v, ".") {
		return "email"
	}
	if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
		return "url"
	}
	return ""
}

func isJSONContentType(ct string) bool {
	if ct == "" {
		// If no content type, try to parse as JSON anyway
		return true
	}
	return strings.Contains(ct, "json")
}
