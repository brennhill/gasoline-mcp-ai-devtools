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
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/util"

	"github.com/dev-console/dev-console/internal/queries"
)

const maxLinkValidationURLs = 1000

// ssrfSafeTransport returns an http.Transport that blocks connections to private IPs.
func ssrfSafeTransport() *http.Transport {
	return newSSRFSafeTransport(nil)
}

// AnalyzeHandler is the function signature for analyze tool handlers.
type AnalyzeHandler func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

// analyzeHandlers maps analyze mode names to their handler functions.
var analyzeHandlers = map[string]AnalyzeHandler{
	// Moved from configure
	"dom": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolQueryDOM(req, args)
	},
	"api_validation": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolValidateAPI(req, args)
	},
	"page_summary": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolAnalyzePageSummary(req, args)
	},

	// Moved from observe
	"performance": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolCheckPerformance(req, args)
	},
	"accessibility": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolRunA11yAudit(req, args)
	},
	"error_clusters": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolAnalyzeErrors(req)
	},
	"history": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolAnalyzeHistory(req, args)
	},
	"security_audit": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolSecurityAudit(req, args)
	},
	"third_party_audit": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolAuditThirdParties(req, args)
	},
	// New
	"link_health": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolAnalyzeLinkHealth(req, args)
	},
	"link_validation": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolValidateLinks(req, args)
	},

	// Draw mode annotations
	"annotations": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGetAnnotations(req, args)
	},
	"annotation_detail": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGetAnnotationDetail(req, args)
	},
	"draw_history": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolListDrawHistory(req, args)
	},
	"draw_session": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGetDrawSession(req, args)
	},
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
// DOM Query (moved from configure)
// ============================================

func (h *ToolHandler) toolQueryDOM(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Selector     string `json:"selector"`
		TabID        int    `json:"tab_id"`
		PierceShadow any    `json:"pierce_shadow"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.Selector == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'selector' is missing", "Add the 'selector' parameter with a CSS selector and call again", withParam("selector"))}
	}
	if params.PierceShadow != nil {
		switch v := params.PierceShadow.(type) {
		case bool:
			// valid
		case string:
			if strings.ToLower(strings.TrimSpace(v)) != "auto" {
				return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Invalid 'pierce_shadow' value: "+v, "Use true, false, or \"auto\"", withParam("pierce_shadow"))}
			}
		default:
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Invalid 'pierce_shadow' type", "Use true, false, or \"auto\"", withParam("pierce_shadow"))}
		}
	}

	// Generate correlation ID for tracking
	correlationID := fmt.Sprintf("dom_%d_%d", time.Now().UnixNano(), randomInt63())

	// Create pending query for DOM query
	query := queries.PendingQuery{
		Type:          "dom",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return h.maybeWaitForCommand(req, correlationID, args, "DOM query queued")
}

func (h *ToolHandler) toolAnalyzePageSummary(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID     int    `json:"tab_id"`
		TimeoutMs int    `json:"timeout_ms,omitempty"`
		World     string `json:"world,omitempty"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.World == "" {
		params.World = "isolated"
	}
	if !validWorldValues[params.World] {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Invalid 'world' value: "+params.World, "Use 'isolated' (default), 'main', or 'auto'", withParam("world"))}
	}

	if params.TimeoutMs <= 0 {
		params.TimeoutMs = 10_000
	}
	if params.TimeoutMs > 30_000 {
		params.TimeoutMs = 30_000
	}

	correlationID := fmt.Sprintf("page_summary_%d_%d", time.Now().UnixNano(), randomInt63())

	// Error impossible: static map with serializable values.
	execParams, _ := json.Marshal(map[string]any{
		"script":     fullSummaryScript(),
		"timeout_ms": params.TimeoutMs,
		"world":      params.World,
		"reason":     "page_summary",
	})

	query := queries.PendingQuery{
		Type:          "execute",
		Params:        execParams,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return h.maybeWaitForCommand(req, correlationID, args, "Page summary queued")
}

// ============================================
// API Validation (moved from configure)
// ============================================

func (h *ToolHandler) toolValidateAPI(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Operation       string   `json:"operation"`
		URLFilter       string   `json:"url"`
		IgnoreEndpoints []string `json:"ignore_endpoints"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	switch params.Operation {
	case "analyze":
		responseData := map[string]any{
			"status":     "ok",
			"operation":  "analyze",
			"violations": []any{},
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API validation", responseData)}

	case "report":
		responseData := map[string]any{
			"status":    "ok",
			"operation": "report",
			"endpoints": []any{},
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API validation", responseData)}

	case "clear":
		clearResult := map[string]any{
			"action": "cleared",
			"status": "ok",
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API validation", clearResult)}

	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "operation parameter must be 'analyze', 'report', or 'clear'", "Use a valid value for 'operation'", withParam("operation"), withHint("analyze, report, or clear"))}
	}
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

	return h.maybeWaitForCommand(req, correlationID, args, "Link health check initiated")
}

// ============================================
// Link Validation (Server-Side)
// ============================================

// LinkValidationParams are parameters for verifying links server-side.
type LinkValidationParams struct {
	URLs       []string `json:"urls"`
	TimeoutMS  int      `json:"timeout_ms,omitempty"`
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

// clampInt clamps v to [min, max], using defaultVal if v is zero.
func clampInt(v, defaultVal, min, max int) int {
	if v == 0 {
		v = defaultVal
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// filterHTTPURLs returns only URLs with http:// or https:// prefix.
func filterHTTPURLs(urls []string) []string {
	valid := make([]string, 0, len(urls))
	for _, u := range urls {
		if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
			valid = append(valid, u)
		}
	}
	return valid
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

	timeoutMS := clampInt(params.TimeoutMS, 15000, 1000, 60000)
	maxWorkers := clampInt(params.MaxWorkers, 20, 1, 100)

	validURLs := filterHTTPURLs(params.URLs)
	if len(validURLs) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "No valid HTTP/HTTPS URLs provided", "URLs must start with http:// or https://", withParam("urls"))}
	}
	if len(validURLs) > maxLinkValidationURLs {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam,
			fmt.Sprintf("Too many URLs: got %d, max %d", len(validURLs), maxLinkValidationURLs),
			fmt.Sprintf("Reduce URLs to %d or fewer and retry", maxLinkValidationURLs),
			withParam("urls"),
		)}
	}

	results := h.validateLinksServerSide(validURLs, timeoutMS, maxWorkers)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Server-side link validation completed", map[string]any{
		"status":  "completed",
		"total":   len(results),
		"results": results,
	})}
}

// validateLinksServerSide performs HTTP HEAD/GET requests to verify link status.
func (h *ToolHandler) validateLinksServerSide(urls []string, timeoutMS int, maxWorkers int) []LinkValidationResult {
	if len(urls) == 0 {
		return []LinkValidationResult{}
	}

	workerCount := maxWorkers
	if workerCount > len(urls) {
		workerCount = len(urls)
	}

	// Share one HTTP client across all workers (connection pooling)
	client := newLinkValidationClient(time.Duration(timeoutMS) * time.Millisecond)

	results := make([]LinkValidationResult, len(urls))
	jobs := make(chan int)

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		util.SafeGo(func() {
			defer wg.Done()
			for idx := range jobs {
				results[idx] = validateSingleLinkWithClient(client, urls[idx])
			}
		})
	}

	for i := range urls {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	return results
}

// newLinkValidationClient creates an HTTP client with SSRF-safe transport and a 5-redirect limit.
func newLinkValidationClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: ssrfSafeTransport(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

// doLinkRequest tries HEAD first, falling back to GET if HEAD fails or returns 405.
func doLinkRequest(client *http.Client, linkURL string) (*http.Response, error) {
	ua := fmt.Sprintf("Gasoline/%s (+https://gasoline.dev)", version)

	req, err := http.NewRequest("HEAD", linkURL, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	req.Header.Set("User-Agent", ua)
	resp, err := client.Do(req) // #nosec G704 -- client uses ssrfSafeTransport() to block private/internal targets

	if err != nil || (resp != nil && resp.StatusCode == http.StatusMethodNotAllowed) {
		if resp != nil {
			_ = resp.Body.Close()
		}
		req, err = http.NewRequest("GET", linkURL, nil)
		if err != nil {
			return nil, fmt.Errorf("invalid URL: %w", err)
		}
		req.Header.Set("User-Agent", ua)
		resp, err = client.Do(req) // #nosec G704 -- client uses ssrfSafeTransport() to block private/internal targets
	}

	return resp, err
}

// classifyHTTPStatus maps an HTTP status code to a link health category.
func classifyHTTPStatus(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "ok"
	case status >= 300 && status < 400:
		return "redirect"
	case status == 401 || status == 403:
		return "requires_auth"
	default:
		return "broken"
	}
}

// buildLinkResult drains the response body and builds a LinkValidationResult from a successful response.
func buildLinkResult(resp *http.Response, url string, timeMS int) LinkValidationResult {
	defer func() { _ = resp.Body.Close() }()

	if _, drainErr := io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20)); drainErr != nil {
		return LinkValidationResult{
			URL:    url,
			Status: resp.StatusCode,
			Code:   "broken",
			TimeMS: timeMS,
			Error:  "failed to read response body: " + drainErr.Error(),
		}
	}

	result := LinkValidationResult{
		URL:    url,
		Status: resp.StatusCode,
		Code:   classifyHTTPStatus(resp.StatusCode),
		TimeMS: timeMS,
	}
	if loc := resp.Header.Get("Location"); loc != "" {
		result.RedirectTo = loc
	}
	return result
}

// validateSingleLinkWithClient performs HTTP verification of a single URL using a shared client.
func validateSingleLinkWithClient(client *http.Client, linkURL string) LinkValidationResult {
	startTime := time.Now()

	resp, err := doLinkRequest(client, linkURL)
	timeMS := int(time.Since(startTime).Milliseconds())

	if err != nil {
		return LinkValidationResult{
			URL:    linkURL,
			Status: 0,
			Code:   "broken",
			TimeMS: timeMS,
			Error:  err.Error(),
		}
	}

	return buildLinkResult(resp, linkURL, timeMS)
}
