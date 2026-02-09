// tools_analyze.go — MCP analyze tool dispatcher and handlers.
// Handles active analysis operations: dom queries, API validation, link health checks,
// performance analysis, accessibility audits, security checks, error analysis, and history analysis.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
)

// AnalyzeHandler is the function signature for analyze tool handlers.
type AnalyzeHandler func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

// analyzeHandlers maps analyze mode names to their handler functions.
var analyzeHandlers = map[string]AnalyzeHandler{
	// Moved from configure
	"dom":              func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolQueryDOM(req, args) },
	"api_validation":   func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolValidateAPI(req, args) },

	// Moved from observe
	"performance":      func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolCheckPerformance(req, args) },
	"accessibility":    func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolRunA11yAudit(req, args) },
	"error_clusters":   func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolAnalyzeErrors(req) },
	"history":          func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolAnalyzeHistory(req, args) },
	"security_audit":   func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolSecurityAudit(req, args) },
	"third_party_audit": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolAuditThirdParties(req, args) },
	"security_diff":    func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolDiffSecurity(req, args) },

	// New
	"link_health":       func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolAnalyzeLinkHealth(req, args) },
	"link_validation":   func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolValidateLinks(req, args) },
}

// getValidAnalyzeModes returns a sorted, comma-separated list of valid analyze modes.
func getValidAnalyzeModes() string {
	modes := make([]string, 0, len(analyzeHandlers))
	for mode := range analyzeHandlers {
		modes = append(modes, mode)
	}
	sort.Strings(modes)
	return strings.Join(modes, ", ")
}

// toolAnalyze dispatches analyze requests based on the 'what' parameter.
func (h *ToolHandler) toolAnalyze(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		What string `json:"what"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.What == "" {
		validModes := getValidAnalyzeModes()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'what' is missing", "Add the 'what' parameter and call again", withParam("what"), withHint("Valid values: "+validModes))}
	}

	handler, ok := analyzeHandlers[params.What]
	if !ok {
		validModes := getValidAnalyzeModes()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown analyze mode: "+params.What, "Use a valid mode from the 'what' enum", withParam("what"), withHint("Valid values: "+validModes))}
	}

	return handler(h, req, args)
}

// ============================================
// Link Health (Placeholder)
// ============================================

// toolAnalyzeLinkHealth checks all links on the current page for health issues.
// This is a placeholder that will be fully implemented in Phase 1.
func (h *ToolHandler) toolAnalyzeLinkHealth(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Generate correlation ID for tracking
	correlationID := fmt.Sprintf("link_health_%d_%d", time.Now().UnixNano(), randomInt63())

	// Create pending query for link health check
	query := queries.PendingQuery{
		Type:          "link_health",
		Params:        args,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Link health check initiated", map[string]any{
		"status":         "queued",
		"correlation_id": correlationID,
		"hint":           "Use observe({what:'command_result', correlation_id:'" + correlationID + "'}) to check the result",
	})}
}

// ============================================
// Link Validation (Server-Side)
// ============================================

// LinkValidationParams are parameters for verifying links server-side.
type LinkValidationParams struct {
	URLs      []string `json:"urls"`
	TimeoutMS int      `json:"timeout_ms,omitempty"`
	MaxWorkers int      `json:"max_workers,omitempty"`
}

// LinkValidationResult contains server-side verification of a single link.
type LinkValidationResult struct {
	URL        string `json:"url"`
	Status     int    `json:"status"`
	Code       string `json:"code"`
	TimeMS     int    `json:"time_ms"`
	RedirectTo string `json:"redirect_to,omitempty"`
	Error      string `json:"error,omitempty"`
}

// toolValidateLinks verifies CORS-blocked URLs using server-side HTTP requests.
// This provides a fallback for links that the browser couldn't check due to CORS.
func (h *ToolHandler) toolValidateLinks(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params LinkValidationParams
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if len(params.URLs) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'urls' is missing or empty", "Provide an array of URLs to validate")}
	}

	// Set defaults
	timeoutMS := params.TimeoutMS
	if timeoutMS == 0 {
		timeoutMS = 15000 // 15 second default
	}
	if timeoutMS < 1000 {
		timeoutMS = 1000 // Minimum 1 second
	}
	if timeoutMS > 60000 {
		timeoutMS = 60000 // Maximum 60 seconds
	}

	maxWorkers := params.MaxWorkers
	if maxWorkers == 0 {
		maxWorkers = 20
	}
	if maxWorkers < 1 {
		maxWorkers = 1
	}
	if maxWorkers > 100 {
		maxWorkers = 100
	}

	// Validate URLs (no malformed ones)
	validURLs := make([]string, 0, len(params.URLs))
	for _, url := range params.URLs {
		if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
			validURLs = append(validURLs, url)
		}
	}

	if len(validURLs) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "No valid HTTP/HTTPS URLs provided", "URLs must start with http:// or https://")}
	}

	// Check links server-side
	results := h.validateLinksServerSide(validURLs, timeoutMS, maxWorkers)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Server-side link validation completed", map[string]any{
		"status":  "completed",
		"total":   len(results),
		"results": results,
	})}
}

// validateLinksServerSide performs HTTP HEAD/GET requests to verify link status.
func (h *ToolHandler) validateLinksServerSide(urls []string, timeoutMS int, maxWorkers int) []LinkValidationResult {
	results := make([]LinkValidationResult, 0, len(urls))
	resultsChan := make(chan LinkValidationResult, len(urls))

	// Semaphore for worker pool
	semaphore := make(chan struct{}, maxWorkers)
	defer close(semaphore)

	for _, url := range urls {
		semaphore <- struct{}{}
		go func(u string) {
			defer func() { <-semaphore }()
			result := validateSingleLink(u, timeoutMS)
			resultsChan <- result
		}(url)
	}

	// Wait for all workers
	for i := 0; i < len(urls); i++ {
		results = append(results, <-resultsChan)
	}

	return results
}

// validateSingleLink performs HTTP verification of a single URL.
func validateSingleLink(url string, timeoutMS int) LinkValidationResult {
	startTime := time.Now()
	timeout := time.Duration(timeoutMS) * time.Millisecond

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Allow up to 5 redirects
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	// Try HEAD request first, fall back to GET if method not allowed
	var resp *http.Response
	var err error
	var code string

	// HEAD request
	req, _ := http.NewRequest("HEAD", url, nil)
	req.Header.Set("User-Agent", "Gasoline/6.0.0 (+https://gasoline.dev)")
	resp, err = client.Do(req)

	// If HEAD fails, try GET (some servers don't support HEAD)
	if err != nil || (resp != nil && resp.StatusCode == http.StatusMethodNotAllowed) {
		if resp != nil {
			resp.Body.Close()
		}
		req, _ = http.NewRequest("GET", url, nil)
		req.Header.Set("User-Agent", "Gasoline/6.0.0 (+https://gasoline.dev)")
		resp, err = client.Do(req)
	}

	timeMS := int(time.Since(startTime).Milliseconds())

	if err != nil {
		return LinkValidationResult{
			URL:    url,
			Status: 0,
			Code:   "broken",
			TimeMS: timeMS,
			Error:  err.Error(),
		}
	}

	defer resp.Body.Close()

	// Drain response body to prevent connection leaks
	io.ReadAll(resp.Body)

	// Categorize by status
	status := resp.StatusCode
	switch {
	case status >= 200 && status < 300:
		code = "ok"
	case status >= 300 && status < 400:
		code = "redirect"
	case status == 401 || status == 403:
		code = "requires_auth"
	case status >= 400:
		code = "broken"
	default:
		code = "broken"
	}

	result := LinkValidationResult{
		URL:    url,
		Status: status,
		Code:   code,
		TimeMS: timeMS,
	}

	if resp.Header.Get("Location") != "" {
		result.RedirectTo = resp.Header.Get("Location")
	}

	return result
}
