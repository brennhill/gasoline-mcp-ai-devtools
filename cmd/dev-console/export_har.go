// export_har.go — HTTP Archive (HAR 1.2) export from captured network traffic.
// Converts the internal network body buffer into standard HAR format,
// compatible with Chrome DevTools, Charles Proxy, and other HAR viewers.
// Design: Filtering by URL, method, and status range before export.
// Auth headers stripped from output. Optional save_to for file output.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ============================================
// HAR 1.2 Types
// ============================================

// HARLog is the top-level HAR structure
type HARLog struct {
	Log HARLogContent `json:"log"`
}

// HARLogContent holds the HAR log metadata and entries
type HARLogContent struct {
	Version string     `json:"version"`
	Creator HARCreator `json:"creator"`
	Entries []HAREntry `json:"entries"`
}

// HARCreator identifies the tool that generated the HAR
type HARCreator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// HAREntry represents a single HTTP request/response pair
type HAREntry struct {
	StartedDateTime string      `json:"startedDateTime"`
	Time            int         `json:"time"`
	Request         HARRequest  `json:"request"`
	Response        HARResponse `json:"response"`
	Cache           struct{}    `json:"cache"`
	Timings         HARTimings  `json:"timings"`
	Comment         string      `json:"comment,omitempty"`
}

// HARRequest represents the HTTP request
type HARRequest struct {
	Method      string       `json:"method"`
	URL         string       `json:"url"`
	HTTPVersion string       `json:"httpVersion"`
	Headers     []HARHeader  `json:"headers"`
	QueryString []HARQuery   `json:"queryString"`
	PostData    *HARPostData `json:"postData,omitempty"`
	HeadersSize int          `json:"headersSize"`
	BodySize    int          `json:"bodySize"`
	Comment     string       `json:"comment,omitempty"`
}

// HARResponse represents the HTTP response
type HARResponse struct {
	Status      int         `json:"status"`
	StatusText  string      `json:"statusText"`
	HTTPVersion string      `json:"httpVersion"`
	Headers     []HARHeader `json:"headers"`
	Content     HARContent  `json:"content"`
	RedirectURL string      `json:"redirectURL"`
	HeadersSize int         `json:"headersSize"`
	BodySize    int         `json:"bodySize"`
	Comment     string      `json:"comment,omitempty"`
}

// HARContent represents the response body content
type HARContent struct {
	Size     int    `json:"size"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text,omitempty"`
}

// HARPostData represents the request body
type HARPostData struct {
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

// HARTimings represents timing information for the request
type HARTimings struct {
	Send    int `json:"send"`
	Wait    int `json:"wait"`
	Receive int `json:"receive"`
}

// HARHeader represents a single HTTP header
type HARHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HARQuery represents a query string parameter
type HARQuery struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HARExportResult is returned when save_to is specified
type HARExportResult struct {
	SavedTo       string `json:"saved_to"`
	EntriesCount  int    `json:"entries_count"`
	FileSizeBytes int    `json:"file_size_bytes"`
}

// ============================================
// Conversion Functions
// ============================================

// networkBodyToHAREntry converts a single NetworkBody to a HAREntry
func networkBodyToHAREntry(body NetworkBody) HAREntry {
	entry := HAREntry{
		StartedDateTime: body.Timestamp,
		Time:            body.Duration,
		Request: HARRequest{
			Method:      body.Method,
			URL:         body.URL,
			HTTPVersion: "HTTP/1.1",
			Headers:     []HARHeader{},
			QueryString: parseQueryString(body.URL),
			HeadersSize: -1,
			BodySize:    len(body.RequestBody),
		},
		Response: HARResponse{
			Status:      body.Status,
			StatusText:  http.StatusText(body.Status),
			HTTPVersion: "HTTP/1.1",
			Headers:     []HARHeader{},
			Content: HARContent{
				Size:     len(body.ResponseBody),
				MimeType: body.ContentType,
				Text:     body.ResponseBody,
			},
			RedirectURL: "",
			HeadersSize: -1,
			BodySize:    len(body.ResponseBody),
		},
		Timings: HARTimings{
			Send:    -1,
			Wait:    body.Duration,
			Receive: -1,
		},
	}

	if body.RequestBody != "" {
		entry.Request.PostData = &HARPostData{
			MimeType: body.ContentType,
			Text:     body.RequestBody,
		}
	}

	if body.RequestTruncated {
		entry.Request.Comment = "Body truncated at 8KB by Gasoline"
	}
	if body.ResponseTruncated {
		entry.Response.Comment = "Body truncated at 16KB by Gasoline"
	}

	return entry
}

// parseQueryString extracts query parameters from a URL
func parseQueryString(rawURL string) []HARQuery {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return []HARQuery{}
	}

	params := parsed.Query()
	if len(params) == 0 {
		return []HARQuery{}
	}

	result := make([]HARQuery, 0, len(params))
	for name, values := range params {
		for _, value := range values {
			result = append(result, HARQuery{Name: name, Value: value})
		}
	}
	return result
}

// ============================================
// Export Functions
// ============================================

// ExportHAR builds a complete HAR log from filtered network bodies.
// Unlike GetNetworkBodies which returns newest-first, ExportHAR returns
// entries in chronological order (oldest first) as required by HAR spec.
func (v *Capture) ExportHAR(filter NetworkBodyFilter) HARLog {
	// Use a high limit to get all bodies for export
	exportFilter := NetworkBodyFilter{
		URLFilter: filter.URLFilter,
		Method:    filter.Method,
		StatusMin: filter.StatusMin,
		StatusMax: filter.StatusMax,
		Limit:     10000, // High limit to get all entries
	}

	bodies := v.GetNetworkBodies(exportFilter)

	// GetNetworkBodies returns newest-first; reverse for chronological order
	reverseSlice(bodies)

	entries := make([]HAREntry, 0, len(bodies))
	for _, body := range bodies {
		entries = append(entries, networkBodyToHAREntry(body))
	}

	return HARLog{
		Log: HARLogContent{
			Version: "1.2",
			Creator: HARCreator{
				Name:    "Gasoline",
				Version: version,
			},
			Entries: entries,
		},
	}
}

// ExportHARToFile exports HAR to a file and returns the result summary
func (v *Capture) ExportHARToFile(filter NetworkBodyFilter, path string) (HARExportResult, error) {
	if !isPathSafe(path) {
		return HARExportResult{}, fmt.Errorf("path not allowed: %s", path)
	}

	harLog := v.ExportHAR(filter)

	data, err := json.MarshalIndent(harLog, "", "  ")
	if err != nil {
		return HARExportResult{}, fmt.Errorf("failed to marshal HAR: %w", err)
	}

	// #nosec G306 -- export files are intentionally world-readable
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return HARExportResult{}, fmt.Errorf("failed to write file: %w", err)
	}

	return HARExportResult{
		SavedTo:       path,
		EntriesCount:  len(harLog.Log.Entries),
		FileSizeBytes: len(data),
	}, nil
}

// ============================================
// Path Validation
// ============================================

// isPathSafe checks if a file path is safe to write to.
// Only allows paths under /tmp, os.TempDir(), or relative paths
// without traversal above the current working directory.
func isPathSafe(path string) bool {
	cleaned := filepath.Clean(path)

	if filepath.IsAbs(cleaned) {
		// Allow /tmp and os.TempDir()
		if strings.HasPrefix(cleaned, "/tmp") {
			return true
		}
		tmpDir := os.TempDir()
		return strings.HasPrefix(cleaned, tmpDir)
	}

	// Relative path - check no traversal above cwd
	return !strings.Contains(cleaned, "..")
}

// ============================================
// MCP Tool Handler
// ============================================

// toolExportHAR handles the export_har MCP tool call
func (h *ToolHandler) toolExportHAR(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		URL       string `json:"url"`
		Method    string `json:"method"`
		StatusMin int    `json:"status_min"`
		StatusMax int    `json:"status_max"`
		SaveTo    string `json:"save_to"`
	}
	_ = json.Unmarshal(args, &arguments)

	filter := NetworkBodyFilter{
		URLFilter: arguments.URL,
		Method:    arguments.Method,
		StatusMin: arguments.StatusMin,
		StatusMax: arguments.StatusMax,
	}

	if arguments.SaveTo != "" {
		// Save to file
		if !isPathSafe(arguments.SaveTo) {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse("Path not allowed: " + arguments.SaveTo)}
		}

		result, err := h.capture.ExportHARToFile(filter, arguments.SaveTo)
		if err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse("Failed to save HAR file: " + err.Error())}
		}

		resultJSON, _ := json.Marshal(result)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(resultJSON))}
	}

	// Return HAR JSON directly
	harLog := h.capture.ExportHAR(filter)
	harJSON, err := json.Marshal(harLog)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse("Failed to marshal HAR: " + err.Error())}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(harJSON))}
}
