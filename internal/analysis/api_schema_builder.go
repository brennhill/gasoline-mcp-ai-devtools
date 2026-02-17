// Purpose: Owns api_schema_builder.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

// api_schema_builder.go — Schema building, OpenAPI stub generation, and statistics.
// Converts accumulated observations into structured API schemas, computes
// timing statistics, detects auth patterns, and generates OpenAPI 3.0 stubs.
package analysis

import (
	"math"
	"sort"
	"strings"
	"time"
)

// ============================================
// Schema Building
// ============================================

// schemaAccum holds intermediate totals during schema building.
type schemaAccum struct {
	totalObservations int
	totalErrors       int
	totalLatency      float64
	latencyCount      int
}

// BuildSchema converts all accumulators into a structured API schema
func (s *SchemaStore) BuildSchema(filter SchemaFilter) APISchema {
	s.mu.RLock()
	defer s.mu.RUnlock()

	endpoints, acc := s.collectEndpoints(filter)

	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].ObservationCount > endpoints[j].ObservationCount
	})

	return APISchema{
		Endpoints:   endpoints,
		WebSockets:  s.buildWSSchemas(),
		AuthPattern: s.detectAuthPattern(),
		Coverage:    buildCoverageStats(endpoints, acc),
	}
}

// collectEndpoints builds endpoint schemas from accumulators that pass the filter.
func (s *SchemaStore) collectEndpoints(filter SchemaFilter) ([]EndpointSchema, schemaAccum) {
	var endpoints []EndpointSchema
	var acc schemaAccum

	for _, a := range s.accumulators {
		if filter.MinObservations > 0 && a.observationCount < filter.MinObservations {
			continue
		}
		if filter.URLFilter != "" && !strings.Contains(a.pathPattern, filter.URLFilter) {
			continue
		}
		endpoints = append(endpoints, s.buildEndpoint(a))
		acc.totalObservations += a.observationCount
		acc.totalErrors += a.errorCount
		for _, l := range a.latencies {
			acc.totalLatency += l
			acc.latencyCount++
		}
	}
	return endpoints, acc
}

// buildCoverageStats computes coverage statistics from endpoints and accumulated totals.
func buildCoverageStats(endpoints []EndpointSchema, acc schemaAccum) CoverageStats {
	coverage := CoverageStats{
		TotalEndpoints: len(endpoints),
		Methods:        make(map[string]int),
	}
	for i := range endpoints {
		coverage.Methods[endpoints[i].Method]++
	}
	if acc.totalObservations > 0 {
		coverage.ErrorRate = float64(acc.totalErrors) / float64(acc.totalObservations) * 100.0
	}
	if acc.latencyCount > 0 {
		coverage.AvgResponseMs = acc.totalLatency / float64(acc.latencyCount)
	}
	return coverage
}

// buildWSSchemas converts internal WebSocket accumulators to output schemas.
func (s *SchemaStore) buildWSSchemas() []WSSchema {
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
	return wsSchemas
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

	authKeywords := []string{"/auth", "/login", "/token"}
	publicKeywords := []string{"/health", "/public"}

	for _, acc := range s.accumulators {
		lowerPattern := strings.ToLower(acc.pathPattern)
		if containsAny(lowerPattern, authKeywords) {
			hasAuthEndpoint = true
			publicPaths = append(publicPaths, acc.pathPattern)
		}
		if containsAny(lowerPattern, publicKeywords) {
			publicPaths = append(publicPaths, acc.pathPattern)
		}
		if !has401 {
			has401 = hasStatusCode(acc.responseShapes, 401)
		}
	}

	if !hasAuthEndpoint && !has401 {
		return nil
	}
	return &AuthPattern{Type: "bearer", Header: "Authorization", AuthRate: 100.0, PublicPaths: publicPaths}
}

// containsAny returns true if s contains any of the substrings.
func containsAny(s string, substrings []string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// hasStatusCode checks if a response shapes map contains the given status code.
func hasStatusCode(shapes map[int]*responseAccumulator, code int) bool {
	_, ok := shapes[code]
	return ok
}

// ============================================
// OpenAPI Stub Generation
// ============================================

// BuildOpenAPIStub generates minimal OpenAPI 3.0 YAML from inferred schema
func (s *SchemaStore) BuildOpenAPIStub(filter SchemaFilter) string {
	schema := s.BuildSchema(filter)

	var b strings.Builder
	b.WriteString("openapi: \"3.0.0\"\ninfo:\n  title: \"Inferred API\"\n  version: \"1.0.0\"\n  description: \"Auto-inferred from observed network traffic\"\npaths:\n")

	pathMethods := groupEndpointsByPath(schema.Endpoints)
	paths := sortedKeys(pathMethods)

	for _, path := range paths {
		b.WriteString("  " + path + ":\n")
		methods := pathMethods[path]
		sort.Slice(methods, func(i, j int) bool { return methods[i].Method < methods[j].Method })
		for i := range methods {
			writeEndpointYAML(&b, &methods[i])
		}
	}
	return b.String()
}

// groupEndpointsByPath groups endpoints by their path pattern.
func groupEndpointsByPath(endpoints []EndpointSchema) map[string][]EndpointSchema {
	pathMethods := make(map[string][]EndpointSchema)
	for i := range endpoints {
		ep := &endpoints[i]
		pathMethods[ep.PathPattern] = append(pathMethods[ep.PathPattern], *ep)
	}
	return pathMethods
}

// sortedKeys returns sorted keys from a map.
func sortedKeys(m map[string][]EndpointSchema) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// writeEndpointYAML writes a single endpoint's YAML to the builder.
func writeEndpointYAML(b *strings.Builder, ep *EndpointSchema) {
	method := strings.ToLower(ep.Method)
	b.WriteString("    " + method + ":\n")
	b.WriteString("      summary: \"" + ep.Method + " " + ep.PathPattern + "\"\n")
	b.WriteString("      responses:\n")
	writeResponseShapes(b, ep)
	writeRequestBody(b, ep)
	writeParameters(b, ep)
}

// writeResponseShapes writes response shape YAML for an endpoint.
func writeResponseShapes(b *strings.Builder, ep *EndpointSchema) {
	if len(ep.ResponseShapes) == 0 {
		b.WriteString("        \"200\":\n          description: \"OK\"\n")
		return
	}
	for status, shape := range ep.ResponseShapes {
		b.WriteString("        \"" + intToString(status) + "\":\n          description: \"Response\"\n")
		if len(shape.Fields) > 0 {
			b.WriteString("          content:\n            application/json:\n              schema:\n                type: object\n                properties:\n")
			for fieldName, fs := range shape.Fields {
				b.WriteString("                  " + fieldName + ":\n                    type: " + mapToOpenAPIType(fs.Type) + "\n")
			}
		}
	}
}

// writeRequestBody writes request body YAML if the endpoint has one.
func writeRequestBody(b *strings.Builder, ep *EndpointSchema) {
	if ep.RequestShape == nil || len(ep.RequestShape.Fields) == 0 {
		return
	}
	b.WriteString("      requestBody:\n        content:\n          application/json:\n            schema:\n              type: object\n              properties:\n")
	for fieldName, fs := range ep.RequestShape.Fields {
		b.WriteString("                " + fieldName + ":\n                  type: " + mapToOpenAPIType(fs.Type) + "\n")
	}
}

// writeParameters writes path and query parameter YAML.
func writeParameters(b *strings.Builder, ep *EndpointSchema) {
	if len(ep.PathParams) == 0 && len(ep.QueryParams) == 0 {
		return
	}
	b.WriteString("      parameters:\n")
	for _, pp := range ep.PathParams {
		b.WriteString("        - name: " + pp.Name + "\n          in: path\n          required: true\n          schema:\n            type: " + mapToOpenAPIType(pp.Type) + "\n")
	}
	for _, qp := range ep.QueryParams {
		b.WriteString("        - name: " + qp.Name + "\n          in: query\n")
		if qp.Required {
			b.WriteString("          required: true\n")
		}
		b.WriteString("          schema:\n            type: " + mapToOpenAPIType(qp.Type) + "\n")
	}
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
