// wire_network.go — Wire types for network telemetry over HTTP.
// Defines the JSON fields sent by the extension for network bodies and waterfall.
// Changes here MUST be mirrored in src/types/wire-network.ts.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
package types

// WireNetworkBody is the canonical wire format for captured network request/response bodies.
type WireNetworkBody struct {
	Method            string `json:"method"`
	URL               string `json:"url"`
	Status            int    `json:"status"`
	RequestBody       string `json:"request_body,omitempty"`
	ResponseBody      string `json:"response_body,omitempty"`
	ContentType       string `json:"content_type,omitempty"`
	Duration          int    `json:"duration,omitempty"`
	RequestTruncated  bool   `json:"request_truncated,omitempty"`
	ResponseTruncated bool   `json:"response_truncated,omitempty"`
	TabId             int    `json:"tab_id,omitempty"`
}

// WireNetworkWaterfallEntry is the canonical wire format for a PerformanceResourceTiming entry.
type WireNetworkWaterfallEntry struct {
	Name            string  `json:"name"`
	URL             string  `json:"url"`
	InitiatorType   string  `json:"initiator_type"`
	Duration        float64 `json:"duration"`
	StartTime       float64 `json:"start_time"`
	FetchStart      float64 `json:"fetch_start"`
	ResponseEnd     float64 `json:"response_end"`
	TransferSize    int     `json:"transfer_size"`
	DecodedBodySize int     `json:"decoded_body_size"`
	EncodedBodySize int     `json:"encoded_body_size"`
	PageURL         string  `json:"page_url,omitempty"`
}

// WireNetworkWaterfallPayload is the top-level shape POSTed to /network-waterfall.
type WireNetworkWaterfallPayload struct {
	Entries []WireNetworkWaterfallEntry `json:"entries"`
	PageURL string                      `json:"page_url"`
}
