// network-types.go — Network waterfall and body types.
// NetworkWaterfallEntry represents browser PerformanceResourceTiming data.
// NetworkBody is an alias to canonical definition in internal/types/network.go.
package capture

import (
	"time"

	"github.com/dev-console/dev-console/internal/types"
)

// NetworkWaterfallEntry represents a single network resource timing entry
// from the browser's PerformanceResourceTiming API
type NetworkWaterfallEntry struct {
	Name            string    `json:"name"`              // Full URL
	URL             string    `json:"url"`               // Same as name
	InitiatorType   string    `json:"initiator_type"`    // snake_case (from browser PerformanceResourceTiming)
	Duration        float64   `json:"duration"`          // snake_case (from browser PerformanceResourceTiming)
	StartTime       float64   `json:"start_time"`        // snake_case (from browser PerformanceResourceTiming)
	FetchStart      float64   `json:"fetch_start"`       // snake_case (from browser PerformanceResourceTiming)
	ResponseEnd     float64   `json:"response_end"`      // snake_case (from browser PerformanceResourceTiming)
	TransferSize    int       `json:"transfer_size"`     // snake_case (from browser PerformanceResourceTiming)
	DecodedBodySize int       `json:"decoded_body_size"` // snake_case (from browser PerformanceResourceTiming)
	EncodedBodySize int       `json:"encoded_body_size"` // snake_case (from browser PerformanceResourceTiming)
	PageURL         string    `json:"page_url,omitempty"`
	Timestamp       time.Time `json:"timestamp,omitempty"` // Server-side timestamp
}

// NetworkWaterfallPayload is POSTed by the extension
type NetworkWaterfallPayload struct {
	Entries []NetworkWaterfallEntry `json:"entries"`
	PageURL string                  `json:"page_url"`
}

// NetworkBody is an alias to canonical definition in internal/types/network.go
type NetworkBody = types.NetworkBody

// NetworkBodyFilter is an alias to canonical definition in internal/types/network.go
type NetworkBodyFilter = types.NetworkBodyFilter
