// api_schema.go — API schema inference from observed network traffic.
// Builds endpoint patterns by normalizing dynamic path segments, tracking
// HTTP methods, status codes, and inferring response shapes from JSON bodies.
// Design: Path normalization detects UUID/numeric segments and replaces with
// :id placeholders. Response shapes use recursive type inference (string,
// number, bool, array, object). Output in gasoline or OpenAPI stub format.
package analysis

import (
	"github.com/dev-console/dev-console/internal/capture"
	"encoding/json"
	"math"
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

// ============================================
// Schema Building
// ============================================

// BuildSchema converts all accumulators into a structured API schema
func (s *SchemaStore) BuildSchema(filter SchemaFilter) APISchema {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var endpoints []EndpointSchema

	totalObservations := 0
	totalErrors := 0
	totalLatency := 0.0
	latencyCount := 0

	for _, acc := range s.accumulators {
		// Apply min observations filter
		if filter.MinObservations > 0 && acc.observationCount < filter.MinObservations {
			continue
		}

		// Apply URL filter
		if filter.URLFilter != "" && !strings.Contains(acc.pathPattern, filter.URLFilter) {
			continue
		}

		ep := s.buildEndpoint(acc)
		endpoints = append(endpoints, ep)

		totalObservations += acc.observationCount
		totalErrors += acc.errorCount
		for _, l := range acc.latencies {
			totalLatency += l
			latencyCount++
		}
	}

	// Sort by observation count (most-used first)
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].ObservationCount > endpoints[j].ObservationCount
	})

	// Build coverage
	coverage := CoverageStats{
		TotalEndpoints: len(endpoints),
		Methods:        make(map[string]int),
	}
	for i := range endpoints {
		coverage.Methods[endpoints[i].Method]++
	}
	if totalObservations > 0 {
		coverage.ErrorRate = float64(totalErrors) / float64(totalObservations) * 100.0
	}
	if latencyCount > 0 {
		coverage.AvgResponseMs = totalLatency / float64(latencyCount)
	}

	// Build WebSocket schemas
	wsSchemas := make([]WSSchema, 0, len(s.wsSchemas))
	for _, ws := range s.wsSchemas {
		wsSchema := WSSchema{
			URL:           ws.url,
			TotalMessages: ws.totalMessages,
			IncomingCount: ws.incomingCount,
			OutgoingCount: ws.outgoingCount,
		}
		for mt := range ws.messageTypes {
			wsSchema.MessageTypes = append(wsSchema.MessageTypes, mt)
		}
		sort.Strings(wsSchema.MessageTypes)
		wsSchemas = append(wsSchemas, wsSchema)
	}

	// Detect auth pattern
	authPattern := s.detectAuthPattern()

	return APISchema{
		Endpoints:   endpoints,
		WebSockets:  wsSchemas,
		AuthPattern: authPattern,
		Coverage:    coverage,
	}
}

func (s *SchemaStore) buildEndpoint(acc *endpointAccumulator) EndpointSchema {
	ep := EndpointSchema{
		Method:           acc.method,
		PathPattern:      acc.pathPattern,
		LastPath:         acc.lastPath,
		ObservationCount: acc.observationCount,
		ResponseShapes:   make(map[int]*BodyShape),
	}

	if !acc.lastSeen.IsZero() {
		ep.LastSeen = acc.lastSeen.Format(time.RFC3339)
	}

	// Build path parameters
	ep.PathParams = s.buildPathParams(acc.pathPattern)

	// Build query parameters
	ep.QueryParams = s.buildQueryParams(acc)

	// Build request shape
	if acc.requestCount > 0 && len(acc.requestFields) > 0 {
		ep.RequestShape = &BodyShape{
			ContentType: "application/json",
			Fields:      s.buildFields(acc.requestFields, acc.requestCount),
			Count:       acc.requestCount,
		}
	}

	// Build response shapes
	for status, ra := range acc.responseShapes {
		if len(ra.fields) > 0 {
			ep.ResponseShapes[status] = &BodyShape{
				ContentType: ra.contentType,
				Fields:      s.buildFields(ra.fields, ra.count),
				Count:       ra.count,
			}
		}
	}

	// Build timing stats
	ep.Timing = computeTimingStats(acc.latencies)

	return ep
}

func (s *SchemaStore) buildPathParams(pattern string) []PathParam {
	segments := strings.Split(pattern, "/")
	var params []PathParam
	for i, seg := range segments {
		switch seg {
		case "{uuid}":
			params = append(params, PathParam{Name: "uuid", Type: "uuid", Position: i})
		case "{id}":
			params = append(params, PathParam{Name: "id", Type: "integer", Position: i})
		case "{hash}":
			params = append(params, PathParam{Name: "hash", Type: "string", Position: i})
		}
	}
	return params
}

func (s *SchemaStore) buildQueryParams(acc *endpointAccumulator) []QueryParam {
	params := make([]QueryParam, 0, len(acc.queryParams))
	for name, pa := range acc.queryParams {
		qp := QueryParam{
			Name:           name,
			ObservedValues: pa.values,
		}

		// Required if present in >90% of observations
		if acc.observationCount > 0 {
			ratio := float64(pa.count) / float64(acc.observationCount)
			qp.Required = ratio > 0.9
		}

		// Type inference
		if pa.allNumeric && len(pa.values) > 0 {
			qp.Type = "integer"
		} else if pa.allBoolean && len(pa.values) > 0 {
			qp.Type = "boolean"
		} else {
			qp.Type = "string"
		}

		params = append(params, qp)
	}

	// Sort for deterministic output
	sort.Slice(params, func(i, j int) bool {
		return params[i].Name < params[j].Name
	})

	return params
}

func (s *SchemaStore) buildFields(fields map[string]*fieldAccumulator, totalCount int) map[string]FieldSchema {
	result := make(map[string]FieldSchema)
	for name, fa := range fields {
		fs := FieldSchema{
			Type:     majorityType(fa.typeCounts),
			Format:   fa.format,
			Observed: fa.observed,
		}

		// Required if present in >90% of observations
		if totalCount > 0 {
			ratio := float64(fa.observed) / float64(totalCount)
			fs.Required = ratio > 0.9
		}

		result[name] = fs
	}
	return result
}

func majorityType(typeCounts map[string]int) string {
	maxCount := 0
	maxType := "string"
	for t, c := range typeCounts {
		if c > maxCount {
			maxCount = c
			maxType = t
		}
	}
	return maxType
}

func computeTimingStats(latencies []float64) TimingStats {
	if len(latencies) == 0 {
		return TimingStats{}
	}

	sorted := make([]float64, len(latencies))
	copy(sorted, latencies)
	sort.Float64s(sorted)

	sum := 0.0
	for _, l := range sorted {
		sum += l
	}

	n := len(sorted)
	return TimingStats{
		Avg: sum / float64(n),
		P50: percentile(sorted, 0.50),
		P95: percentile(sorted, 0.95),
		Max: sorted[n-1],
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	index := p * float64(len(sorted)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))
	if lower == upper {
		return sorted[lower]
	}
	fraction := index - float64(lower)
	return sorted[lower]*(1-fraction) + sorted[upper]*fraction
}

// ============================================
// Auth Pattern Detection
// ============================================

func (s *SchemaStore) detectAuthPattern() *AuthPattern {
	hasAuthEndpoint := false
	has401 := false
	var publicPaths []string
	totalRequests := 0

	for _, acc := range s.accumulators {
		totalRequests += acc.observationCount
		lowerPattern := strings.ToLower(acc.pathPattern)

		if strings.Contains(lowerPattern, "/auth") ||
			strings.Contains(lowerPattern, "/login") ||
			strings.Contains(lowerPattern, "/token") {
			hasAuthEndpoint = true
			publicPaths = append(publicPaths, acc.pathPattern)
		}

		if strings.Contains(lowerPattern, "/health") ||
			strings.Contains(lowerPattern, "/public") {
			publicPaths = append(publicPaths, acc.pathPattern)
		}

		for status := range acc.responseShapes {
			if status == 401 {
				has401 = true
			}
		}
	}

	if !hasAuthEndpoint && !has401 {
		return nil
	}

	return &AuthPattern{
		Type:        "bearer",
		Header:      "Authorization",
		AuthRate:    100.0, // We can't see headers, assume authenticated
		PublicPaths: publicPaths,
	}
}

// ============================================
// OpenAPI Stub Generation
// ============================================

// BuildOpenAPIStub generates minimal OpenAPI 3.0 YAML from inferred schema
func (s *SchemaStore) BuildOpenAPIStub(filter SchemaFilter) string {
	schema := s.BuildSchema(filter)

	var b strings.Builder
	b.WriteString("openapi: \"3.0.0\"\n")
	b.WriteString("info:\n")
	b.WriteString("  title: \"Inferred API\"\n")
	b.WriteString("  version: \"1.0.0\"\n")
	b.WriteString("  description: \"Auto-inferred from observed network traffic\"\n")
	b.WriteString("paths:\n")

	// Group endpoints by path pattern
	pathMethods := make(map[string][]EndpointSchema)
	for i := range schema.Endpoints {
		ep := &schema.Endpoints[i]
		pathMethods[ep.PathPattern] = append(pathMethods[ep.PathPattern], *ep)
	}

	// Sort paths for deterministic output
	paths := make([]string, 0, len(pathMethods))
	for p := range pathMethods {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, path := range paths {
		b.WriteString("  " + path + ":\n")
		methods := pathMethods[path]
		sort.Slice(methods, func(i, j int) bool {
			return methods[i].Method < methods[j].Method
		})
		for i := range methods {
			ep := &methods[i]
			method := strings.ToLower(ep.Method)
			b.WriteString("    " + method + ":\n")
			b.WriteString("      summary: \"" + ep.Method + " " + ep.PathPattern + "\"\n")
			b.WriteString("      responses:\n")

			if len(ep.ResponseShapes) > 0 {
				for status, shape := range ep.ResponseShapes {
					b.WriteString("        \"" + intToString(status) + "\":\n")
					b.WriteString("          description: \"Response\"\n")
					if len(shape.Fields) > 0 {
						b.WriteString("          content:\n")
						b.WriteString("            application/json:\n")
						b.WriteString("              schema:\n")
						b.WriteString("                type: object\n")
						b.WriteString("                properties:\n")
						for fieldName, fs := range shape.Fields {
							b.WriteString("                  " + fieldName + ":\n")
							b.WriteString("                    type: " + mapToOpenAPIType(fs.Type) + "\n")
						}
					}
				}
			} else {
				b.WriteString("        \"200\":\n")
				b.WriteString("          description: \"OK\"\n")
			}

			if ep.RequestShape != nil && len(ep.RequestShape.Fields) > 0 {
				b.WriteString("      requestBody:\n")
				b.WriteString("        content:\n")
				b.WriteString("          application/json:\n")
				b.WriteString("            schema:\n")
				b.WriteString("              type: object\n")
				b.WriteString("              properties:\n")
				for fieldName, fs := range ep.RequestShape.Fields {
					b.WriteString("                " + fieldName + ":\n")
					b.WriteString("                  type: " + mapToOpenAPIType(fs.Type) + "\n")
				}
			}

			if len(ep.PathParams) > 0 || len(ep.QueryParams) > 0 {
				b.WriteString("      parameters:\n")
				for _, pp := range ep.PathParams {
					b.WriteString("        - name: " + pp.Name + "\n")
					b.WriteString("          in: path\n")
					b.WriteString("          required: true\n")
					b.WriteString("          schema:\n")
					b.WriteString("            type: " + mapToOpenAPIType(pp.Type) + "\n")
				}
				for _, qp := range ep.QueryParams {
					b.WriteString("        - name: " + qp.Name + "\n")
					b.WriteString("          in: query\n")
					if qp.Required {
						b.WriteString("          required: true\n")
					}
					b.WriteString("          schema:\n")
					b.WriteString("            type: " + mapToOpenAPIType(qp.Type) + "\n")
				}
			}
		}
	}

	return b.String()
}

func mapToOpenAPIType(t string) string {
	switch t {
	case "integer":
		return "integer"
	case "number":
		return "number"
	case "boolean":
		return "boolean"
	case "array":
		return "array"
	case "object":
		return "object"
	case "uuid":
		return "string"
	default:
		return "string"
	}
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	if neg {
		result = "-" + result
	}
	return result
}
