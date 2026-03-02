// Purpose: Converts captured NetworkBody entries into HAR 1.2 format for import into browser DevTools and HAR consumers.
// Docs: docs/features/feature/har-export/index.md

// export_har.go — HAR 1.2 export from captured network data.
// Converts NetworkBody entries to HTTP Archive format for import into
// browser DevTools, Charles Proxy, and other HAR consumers.
// Design: Standalone functions, no Capture dependency. Called by toolExportHAR handler.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
// SPEC:HAR — HAR 1.2 fields use camelCase per http://www.softwareishard.com/blog/har-12-spec/
package export

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/dev-console/dev-console/internal/types"
)

// ============================================
// HAR 1.2 Types
// ============================================

// HARLog is the top-level HAR structure.
type HARLog struct {
	Log HARLogInner `json:"log"` // SPEC:HAR
}

// HARLogInner contains the HAR version, creator, and entries.
type HARLogInner struct {
	Version string     `json:"version"` // SPEC:HAR
	Creator HARCreator `json:"creator"` // SPEC:HAR
	Entries []HAREntry `json:"entries"` // SPEC:HAR
}

// HARCreator identifies the tool that generated the HAR.
type HARCreator struct {
	Name    string `json:"name"`    // SPEC:HAR
	Version string `json:"version"` // SPEC:HAR
}

// HAREntry represents a single HTTP request/response pair.
type HAREntry struct {
	StartedDateTime string      `json:"startedDateTime"`   // SPEC:HAR
	Time            int         `json:"time"`              // SPEC:HAR — total elapsed time in ms
	Request         HARRequest  `json:"request"`           // SPEC:HAR
	Response        HARResponse `json:"response"`          // SPEC:HAR
	Timings         HARTimings  `json:"timings"`           // SPEC:HAR
	Comment         string      `json:"comment,omitempty"` // SPEC:HAR
}

// HARRequest represents an HTTP request.
type HARRequest struct {
	Method      string         `json:"method"`             // SPEC:HAR
	URL         string         `json:"url"`                // SPEC:HAR
	HTTPVersion string         `json:"httpVersion"`        // SPEC:HAR
	Headers     []HARNameValue `json:"headers"`            // SPEC:HAR
	QueryString []HARNameValue `json:"queryString"`        // SPEC:HAR
	PostData    *HARPostData   `json:"postData,omitempty"` // SPEC:HAR
	HeadersSize int            `json:"headersSize"`        // SPEC:HAR
	BodySize    int            `json:"bodySize"`           // SPEC:HAR
	Comment     string         `json:"comment,omitempty"`  // SPEC:HAR
}

// HARResponse represents an HTTP response.
type HARResponse struct {
	Status      int            `json:"status"`            // SPEC:HAR
	StatusText  string         `json:"statusText"`        // SPEC:HAR
	HTTPVersion string         `json:"httpVersion"`       // SPEC:HAR
	Headers     []HARNameValue `json:"headers"`           // SPEC:HAR
	Content     HARContent     `json:"content"`           // SPEC:HAR
	HeadersSize int            `json:"headersSize"`       // SPEC:HAR
	BodySize    int            `json:"bodySize"`          // SPEC:HAR
	Comment     string         `json:"comment,omitempty"` // SPEC:HAR
}

// HARContent represents response body content.
type HARContent struct {
	Size     int    `json:"size"`           // SPEC:HAR
	MimeType string `json:"mimeType"`       // SPEC:HAR
	Text     string `json:"text,omitempty"` // SPEC:HAR
}

// HARTimings contains timing breakdown for the request.
type HARTimings struct {
	Send    int `json:"send"`    // SPEC:HAR
	Wait    int `json:"wait"`    // SPEC:HAR
	Receive int `json:"receive"` // SPEC:HAR
}

// HARNameValue is a generic name/value pair for headers, query params, etc.
type HARNameValue struct {
	Name  string `json:"name"`  // SPEC:HAR
	Value string `json:"value"` // SPEC:HAR
}

// HARPostData represents request body data.
type HARPostData struct {
	MimeType string `json:"mimeType"` // SPEC:HAR
	Text     string `json:"text"`     // SPEC:HAR
}

// HARExportResult is the response when saving HAR to a file.
type HARExportResult struct {
	SavedTo       string `json:"saved_to"`
	EntriesCount  int    `json:"entries_count"`
	FileSizeBytes int64  `json:"file_size_bytes"`
}

// ============================================
// Export Functions
// ============================================

// ExportHAR converts captured network bodies to a HAR 1.2 log, applying filters.
// Entries are returned in chronological order (oldest first).
func ExportHAR(bodies []types.NetworkBody, filter types.NetworkBodyFilter, creatorVersion string) HARLog {
	entries := make([]HAREntry, 0)
	for _, body := range bodies {
		if !matchesHARFilter(body, filter) {
			continue
		}
		entries = append(entries, networkBodyToHAREntry(body))
	}
	return buildHARLog(entries, creatorVersion)
}

// ExportHARToFile exports HAR to a JSON file on disk.
func ExportHARToFile(bodies []types.NetworkBody, filter types.NetworkBodyFilter, creatorVersion string, path string) (HARExportResult, error) {
	harLog := ExportHAR(bodies, filter, creatorVersion)
	return writeHARToFile(harLog, path)
}

// ExportHARMerged merges NetworkBody entries with NetworkWaterfall entries into a single HAR log.
// Bodies provide full request/response data. Waterfall entries that match a body URL enrich its
// timings. Waterfall-only entries become lightweight HAR entries with timing and size but no body.
func ExportHARMerged(bodies []types.NetworkBody, waterfall []types.NetworkWaterfallEntry, filter types.NetworkBodyFilter, creatorVersion string) HARLog {
	// Build map of entries keyed by URL from bodies.
	entryMap := make(map[string]*HAREntry, len(bodies))
	var entries []HAREntry

	for _, body := range bodies {
		if !matchesHARFilter(body, filter) {
			continue
		}
		entry := networkBodyToHAREntry(body)
		entries = append(entries, entry)
		entryMap[body.URL] = &entries[len(entries)-1]
	}

	// Merge waterfall entries.
	for _, wf := range waterfall {
		if !matchesWaterfallFilter(wf, filter) {
			continue
		}
		if existing, ok := entryMap[wf.URL]; ok {
			// Enrich timing on the existing body entry.
			enrichTimingsFromWaterfall(existing, wf)
		} else {
			// New waterfall-only entry.
			entries = append(entries, waterfallToHAREntry(wf))
		}
	}

	// Sort entries chronologically by StartedDateTime.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].StartedDateTime < entries[j].StartedDateTime
	})

	if entries == nil {
		entries = make([]HAREntry, 0)
	}

	return buildHARLog(entries, creatorVersion)
}

// ExportHARMergedToFile exports merged HAR to a JSON file on disk.
func ExportHARMergedToFile(bodies []types.NetworkBody, waterfall []types.NetworkWaterfallEntry, filter types.NetworkBodyFilter, creatorVersion string, path string) (HARExportResult, error) {
	harLog := ExportHARMerged(bodies, waterfall, filter, creatorVersion)
	return writeHARToFile(harLog, path)
}

func buildHARLog(entries []HAREntry, creatorVersion string) HARLog {
	return HARLog{
		Log: HARLogInner{
			Version: "1.2",
			Creator: HARCreator{Name: "Gasoline Agentic Browser", Version: creatorVersion},
			Entries: entries,
		},
	}
}

func writeHARToFile(harLog HARLog, path string) (HARExportResult, error) {
	if !isPathSafe(path) {
		return HARExportResult{}, fmt.Errorf("unsafe path: %s", path)
	}

	data, err := json.MarshalIndent(harLog, "", "  ")
	if err != nil {
		return HARExportResult{}, fmt.Errorf("failed to marshal HAR: %w", err)
	}

	if err := writeHARData(path, data); err != nil {
		return HARExportResult{}, err
	}

	return HARExportResult{
		SavedTo:       path,
		EntriesCount:  len(harLog.Log.Entries),
		FileSizeBytes: int64(len(data)),
	}, nil
}
