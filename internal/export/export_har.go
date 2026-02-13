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
	"net/url"
	"os"
	"path/filepath"
	"strings"

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
	StartedDateTime string      `json:"startedDateTime"` // SPEC:HAR
	Time            int         `json:"time"`            // SPEC:HAR — total elapsed time in ms
	Request         HARRequest  `json:"request"`         // SPEC:HAR
	Response        HARResponse `json:"response"`        // SPEC:HAR
	Timings         HARTimings  `json:"timings"`         // SPEC:HAR
}

// HARRequest represents an HTTP request.
type HARRequest struct {
	Method      string         `json:"method"`      // SPEC:HAR
	URL         string         `json:"url"`         // SPEC:HAR
	HTTPVersion string         `json:"httpVersion"` // SPEC:HAR
	Headers     []HARNameValue `json:"headers"`     // SPEC:HAR
	QueryString []HARNameValue `json:"queryString"` // SPEC:HAR
	PostData    *HARPostData   `json:"postData,omitempty"` // SPEC:HAR
	HeadersSize int            `json:"headersSize"` // SPEC:HAR
	BodySize    int            `json:"bodySize"`    // SPEC:HAR
	Comment     string         `json:"comment,omitempty"` // SPEC:HAR
}

// HARResponse represents an HTTP response.
type HARResponse struct {
	Status      int            `json:"status"`      // SPEC:HAR
	StatusText  string         `json:"statusText"`  // SPEC:HAR
	HTTPVersion string         `json:"httpVersion"` // SPEC:HAR
	Headers     []HARNameValue `json:"headers"`     // SPEC:HAR
	Content     HARContent     `json:"content"`     // SPEC:HAR
	HeadersSize int            `json:"headersSize"` // SPEC:HAR
	BodySize    int            `json:"bodySize"`    // SPEC:HAR
	Comment     string         `json:"comment,omitempty"` // SPEC:HAR
}

// HARContent represents response body content.
type HARContent struct {
	Size     int    `json:"size"`     // SPEC:HAR
	MimeType string `json:"mimeType"` // SPEC:HAR
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

	return HARLog{
		Log: HARLogInner{
			Version: "1.2",
			Creator: HARCreator{Name: "Gasoline", Version: creatorVersion},
			Entries: entries,
		},
	}
}

// ExportHARToFile exports HAR to a JSON file on disk.
func ExportHARToFile(bodies []types.NetworkBody, filter types.NetworkBodyFilter, creatorVersion string, path string) (HARExportResult, error) {
	if !isPathSafe(path) {
		return HARExportResult{}, fmt.Errorf("unsafe path: %s", path)
	}

	harLog := ExportHAR(bodies, filter, creatorVersion)
	data, err := json.MarshalIndent(harLog, "", "  ")
	if err != nil {
		return HARExportResult{}, fmt.Errorf("failed to marshal HAR: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return HARExportResult{}, fmt.Errorf("failed to write file: %w", err)
	}

	return HARExportResult{
		SavedTo:       path,
		EntriesCount:  len(harLog.Log.Entries),
		FileSizeBytes: int64(len(data)),
	}, nil
}

// ============================================
// Conversion
// ============================================

// networkBodyToHAREntry converts a single NetworkBody to a HAR entry.
func networkBodyToHAREntry(body types.NetworkBody) HAREntry {
	entry := HAREntry{
		StartedDateTime: body.Timestamp,
		Time:            body.Duration,
		Request:         buildHARRequest(body),
		Response:        buildHARResponse(body),
		Timings: HARTimings{
			Send:    -1,
			Wait:    body.Duration,
			Receive: -1,
		},
	}
	return entry
}

func buildHARRequest(body types.NetworkBody) HARRequest {
	req := HARRequest{
		Method:      body.Method,
		URL:         body.URL,
		HTTPVersion: "HTTP/1.1",
		Headers:     make([]HARNameValue, 0),
		QueryString: parseQueryString(body.URL),
		HeadersSize: -1,
		BodySize:    0,
	}

	if body.RequestBody != "" {
		req.PostData = &HARPostData{
			MimeType: body.ContentType,
			Text:     body.RequestBody,
		}
		req.BodySize = len(body.RequestBody)
	}

	if body.RequestTruncated {
		req.Comment = "Body truncated at 8KB by Gasoline"
	}

	return req
}

func buildHARResponse(body types.NetworkBody) HARResponse {
	resp := HARResponse{
		Status:      body.Status,
		StatusText:  httpStatusText(body.Status),
		HTTPVersion: "HTTP/1.1",
		Headers:     make([]HARNameValue, 0),
		Content: HARContent{
			Size:     len(body.ResponseBody),
			MimeType: body.ContentType,
			Text:     body.ResponseBody,
		},
		HeadersSize: -1,
		BodySize:    len(body.ResponseBody),
	}

	if body.ResponseTruncated {
		resp.Comment = "Body truncated at 16KB by Gasoline"
	}

	return resp
}

// ============================================
// Helpers
// ============================================

// parseQueryString extracts query parameters from a URL as name/value pairs.
func parseQueryString(rawURL string) []HARNameValue {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return make([]HARNameValue, 0)
	}
	params := parsed.Query()
	if len(params) == 0 {
		return make([]HARNameValue, 0)
	}
	result := make([]HARNameValue, 0, len(params))
	for name, values := range params {
		for _, val := range values {
			result = append(result, HARNameValue{Name: name, Value: val})
		}
	}
	return result
}

// httpStatusText returns the standard text for an HTTP status code.
func httpStatusText(code int) string {
	switch code {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 204:
		return "No Content"
	case 301:
		return "Moved Permanently"
	case 302:
		return "Found"
	case 304:
		return "Not Modified"
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 500:
		return "Internal Server Error"
	case 502:
		return "Bad Gateway"
	case 503:
		return "Service Unavailable"
	default:
		return ""
	}
}

// isPathSafe rejects path traversal and absolute paths outside temp directories.
func isPathSafe(path string) bool {
	if strings.Contains(path, "..") {
		return false
	}
	if filepath.IsAbs(path) {
		tmpDir := os.TempDir()
		return strings.HasPrefix(path, "/tmp/") ||
			strings.HasPrefix(path, "/private/tmp/") ||
			strings.HasPrefix(path, tmpDir+"/")
	}
	return true
}

// matchesHARFilter checks if a NetworkBody passes the filter criteria.
func matchesHARFilter(body types.NetworkBody, filter types.NetworkBodyFilter) bool {
	if filter.URLFilter != "" && !strings.Contains(strings.ToLower(body.URL), strings.ToLower(filter.URLFilter)) {
		return false
	}
	if filter.Method != "" && !strings.EqualFold(body.Method, filter.Method) {
		return false
	}
	if filter.StatusMin > 0 && body.Status < filter.StatusMin {
		return false
	}
	if filter.StatusMax > 0 && body.Status > filter.StatusMax {
		return false
	}
	return true
}
