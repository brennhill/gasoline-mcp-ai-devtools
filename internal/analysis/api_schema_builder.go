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
