// Purpose: Implements core API schema shape-building and normalization helpers.
// Why: Keeps inferred-schema generation deterministic and reusable across analysis paths.
// Docs: docs/features/feature/api-schema/index.md

package analysis

import (
	"sort"
	"strings"
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

// BuildSchema converts all accumulators into a structured API schema.
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

	for _, endpointAcc := range s.accumulators {
		if filter.MinObservations > 0 && endpointAcc.observationCount < filter.MinObservations {
			continue
		}
		if filter.URLFilter != "" && !strings.Contains(endpointAcc.pathPattern, filter.URLFilter) {
			continue
		}
		endpoints = append(endpoints, s.buildEndpoint(endpointAcc))
		acc.totalObservations += endpointAcc.observationCount
		acc.totalErrors += endpointAcc.errorCount
		for _, latency := range endpointAcc.latencies {
			acc.totalLatency += latency
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
