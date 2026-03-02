// Purpose: Implements WebSocket schema conversion and timing percentile utilities.
// Why: Keeps statistical calculations and ws-shape assembly separate from endpoint builders.
// Docs: docs/features/feature/api-schema/index.md

package analysis

import (
	"math"
	"sort"
)

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
		for messageType := range ws.messageTypes {
			wsSchema.MessageTypes = append(wsSchema.MessageTypes, messageType)
		}
		sort.Strings(wsSchema.MessageTypes)
		wsSchemas = append(wsSchemas, wsSchema)
	}
	return wsSchemas
}

func computeTimingStats(latencies []float64) TimingStats {
	if len(latencies) == 0 {
		return TimingStats{}
	}

	sorted := make([]float64, len(latencies))
	copy(sorted, latencies)
	sort.Float64s(sorted)

	sum := 0.0
	for _, latency := range sorted {
		sum += latency
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
