// Purpose: Implements endpoint-level schema shape builders (path/query/body fields).
// Why: Keeps endpoint modeling logic separate from top-level orchestration and timing helpers.
// Docs: docs/features/feature/api-schema/index.md

package analysis

import (
	"sort"
	"strings"
	"time"
)

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

	ep.PathParams = s.buildPathParams(acc.pathPattern)
	ep.QueryParams = s.buildQueryParams(acc)

	if acc.requestCount > 0 && len(acc.requestFields) > 0 {
		ep.RequestShape = &BodyShape{
			ContentType: "application/json",
			Fields:      s.buildFields(acc.requestFields, acc.requestCount),
			Count:       acc.requestCount,
		}
	}

	for status, responseAcc := range acc.responseShapes {
		if len(responseAcc.fields) > 0 {
			ep.ResponseShapes[status] = &BodyShape{
				ContentType: responseAcc.contentType,
				Fields:      s.buildFields(responseAcc.fields, responseAcc.count),
				Count:       responseAcc.count,
			}
		}
	}

	ep.Timing = computeTimingStats(acc.latencies)

	return ep
}

func (s *SchemaStore) buildPathParams(pattern string) []PathParam {
	segments := strings.Split(pattern, "/")
	var params []PathParam
	for i, segment := range segments {
		switch segment {
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
	for name, paramAcc := range acc.queryParams {
		qp := QueryParam{
			Name:           name,
			ObservedValues: paramAcc.values,
		}

		if acc.observationCount > 0 {
			ratio := float64(paramAcc.count) / float64(acc.observationCount)
			qp.Required = ratio > 0.9
		}

		if paramAcc.allNumeric && len(paramAcc.values) > 0 {
			qp.Type = "integer"
		} else if paramAcc.allBoolean && len(paramAcc.values) > 0 {
			qp.Type = "boolean"
		} else {
			qp.Type = "string"
		}

		params = append(params, qp)
	}

	sort.Slice(params, func(i, j int) bool {
		return params[i].Name < params[j].Name
	})

	return params
}

func (s *SchemaStore) buildFields(fields map[string]*fieldAccumulator, totalCount int) map[string]FieldSchema {
	result := make(map[string]FieldSchema)
	for name, fieldAcc := range fields {
		fs := FieldSchema{
			Type:     majorityType(fieldAcc.typeCounts),
			Format:   fieldAcc.format,
			Observed: fieldAcc.observed,
		}

		if totalCount > 0 {
			ratio := float64(fieldAcc.observed) / float64(totalCount)
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
