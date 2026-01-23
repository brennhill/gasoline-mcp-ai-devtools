package main

// ============================================
// v5 TDD Stubs — pre-existing tests from main
// that have not been implemented yet.
// These stubs allow the package to compile.
// ============================================

// TimelineFilter defines filtering for session timeline
type TimelineFilter struct {
	LastNActions int
	URLFilter    string
	Include      []string
}

// TimelineEntry represents a single entry in the session timeline
type TimelineEntry struct {
	Timestamp     int64                  `json:"timestamp"`
	Kind          string                 `json:"kind"`
	Type          string                 `json:"type"`
	URL           string                 `json:"url,omitempty"`
	ToURL         string                 `json:"toUrl,omitempty"`
	Selectors     map[string]interface{} `json:"selectors,omitempty"`
	Method        string                 `json:"method,omitempty"`
	Status        int                    `json:"status,omitempty"`
	ContentType   string                 `json:"contentType,omitempty"`
	ResponseShape interface{}            `json:"responseShape,omitempty"`
	Level         string                 `json:"level,omitempty"`
	Message       string                 `json:"message,omitempty"`
	Value         string                 `json:"value,omitempty"`
}

// TimelineSummary contains aggregate counts for the timeline
type TimelineSummary struct {
	Actions         int   `json:"actions"`
	NetworkRequests int   `json:"networkRequests"`
	ConsoleErrors   int   `json:"consoleErrors"`
	DurationMs      int64 `json:"durationMs"`
}

// SessionTimelineResponse is the response from get_session_timeline
type SessionTimelineResponse struct {
	Timeline []TimelineEntry `json:"timeline"`
	Summary  TimelineSummary `json:"summary"`
}

// TestGenerationOptions controls test script generation
type TestGenerationOptions struct {
	TestName           string
	AssertNetwork      bool
	AssertNoErrors     bool
	AssertResponseShape bool
	BaseURL            string
}

// normalizeTimestamp converts a timestamp string to milliseconds (stub)
func normalizeTimestamp(s string) int64 {
	return 0
}

// GetSessionTimeline returns a merged, sorted timeline of session events (stub)
func (v *V4Server) GetSessionTimeline(filter TimelineFilter, logs []LogEntry) SessionTimelineResponse {
	return SessionTimelineResponse{}
}

// generateTestScript generates a Playwright test script from a timeline (stub)
func generateTestScript(timeline []TimelineEntry, opts TestGenerationOptions) string {
	return ""
}
