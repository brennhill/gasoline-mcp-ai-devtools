// Purpose: Implements analyze tool handlers and response shaping.
// Docs: docs/features/feature/analyze-tool/index.md

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

	"github.com/dev-console/dev-console/internal/analysis"
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
		Selector string `json:"selector"`
		TabID    int    `json:"tab_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.Selector == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'selector' is missing", "Add the 'selector' parameter with a CSS selector and call again", withParam("selector"))}
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

const pageSummaryScript = `(function () {
  function cleanText(value, maxLen) {
    var text = (value || '').replace(/[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]/g, '').replace(/\s+/g, ' ').trim();
    if (maxLen > 0 && text.length > maxLen) {
      return text.slice(0, maxLen);
    }
    return text;
  }

  function absoluteHref(value) {
    try {
      return new URL(value || '', window.location.href).href;
    } catch (_err) {
      return value || '';
    }
  }

  function visibleInteractiveCount() {
    var nodes = document.querySelectorAll(
      'a[href],button,input:not([type="hidden"]),select,textarea,[role="button"],[role="link"],[tabindex]'
    );
    var count = 0;
    for (var i = 0; i < nodes.length; i++) {
      var node = nodes[i];
      if (node.disabled) continue;
      var style = window.getComputedStyle(node);
      if (style.display === 'none' || style.visibility === 'hidden') continue;
      var rect = node.getBoundingClientRect();
      if (rect.width <= 0 || rect.height <= 0) continue;
      count += 1;
    }
    return count;
  }

  function findMainNode() {
    var candidates = [
      'main',
      'article',
      '[role="main"]',
      '#main',
      '.main',
      '.content',
      '#content',
      '.article',
      '.post',
      '.results'
    ];
    for (var i = 0; i < candidates.length; i++) {
      var node = document.querySelector(candidates[i]);
      if (!node) continue;
      var text = cleanText(node.innerText || node.textContent || '', 0);
      if (text.length > 120) {
        return node;
      }
    }
    return document.body || document.documentElement;
  }

  function classifyPage(forms, interactiveCount, linkCount, paragraphCount, headingCount, previewText) {
    var hasSearchInput = !!document.querySelector(
      'input[type="search"], input[name*="search" i], input[placeholder*="search" i]'
    );
    var likelySearchURL = /[?&](q|query|search)=/i.test(window.location.search);
    var hasArticle = document.querySelectorAll('article').length > 0;
    var hasTable = document.querySelectorAll('table').length > 0;
    var totalFormFields = 0;
    for (var i = 0; i < forms.length; i++) {
      totalFormFields += (forms[i].fields || []).length;
    }

    if (hasSearchInput && (likelySearchURL || linkCount > 10)) {
      return 'search_results';
    }
    if (forms.length > 0 && totalFormFields >= 3 && paragraphCount < 8) {
      return 'form';
    }
    if (hasArticle || (paragraphCount >= 8 && linkCount < paragraphCount * 2)) {
      return 'article';
    }
    if (hasTable || (interactiveCount > 25 && headingCount >= 2)) {
      return 'dashboard';
    }
    if (linkCount > 30 && paragraphCount < 10) {
      return 'link_list';
    }
    if (previewText.length < 80 && interactiveCount > 10) {
      return 'app';
    }
    return 'generic';
  }

  var headingNodes = document.querySelectorAll('h1, h2, h3');
  var headings = [];
  for (var i = 0; i < headingNodes.length && headings.length < 30; i++) {
    var heading = headingNodes[i];
    var text = cleanText(heading.innerText || heading.textContent || '', 200);
    if (!text) continue;
    headings.push(heading.tagName.toLowerCase() + ': ' + text);
  }

  var navCandidates = document.querySelectorAll('nav a[href], header a[href], [role="navigation"] a[href]');
  var navLinks = [];
  var seenNav = {};
  for (var j = 0; j < navCandidates.length && navLinks.length < 25; j++) {
    var link = navCandidates[j];
    var linkText = cleanText(link.innerText || link.textContent || '', 80);
    var href = absoluteHref(link.getAttribute('href'));
    if (!href) continue;
    var key = linkText + '|' + href;
    if (seenNav[key]) continue;
    seenNav[key] = true;
    navLinks.push({ text: linkText, href: href });
  }

  var forms = [];
  var formNodes = document.querySelectorAll('form');
  for (var k = 0; k < formNodes.length && forms.length < 10; k++) {
    var form = formNodes[k];
    var fieldNodes = form.querySelectorAll('input, select, textarea');
    var fields = [];
    var seenFields = {};
    for (var m = 0; m < fieldNodes.length && fields.length < 25; m++) {
      var field = fieldNodes[m];
      var candidate =
        field.getAttribute('name') ||
        field.getAttribute('id') ||
        field.getAttribute('aria-label') ||
        field.getAttribute('type') ||
        field.tagName.toLowerCase();
      candidate = cleanText(candidate, 60);
      if (!candidate || seenFields[candidate]) continue;
      seenFields[candidate] = true;
      fields.push(candidate);
    }
    forms.push({
      action: absoluteHref(form.getAttribute('action') || window.location.href),
      method: (form.getAttribute('method') || 'GET').toUpperCase(),
      fields: fields
    });
  }

  var mainNode = findMainNode();
  var mainText = cleanText(mainNode ? mainNode.innerText || mainNode.textContent || '' : '', 20000);
  var preview = mainText.slice(0, 500);
  var wordCount = mainText ? mainText.split(/\s+/).filter(Boolean).length : 0;

  var linkCount = document.querySelectorAll('a[href]').length;
  var paragraphCount = document.querySelectorAll('p').length;
  var interactiveCount = visibleInteractiveCount();
  var pageType = classifyPage(forms, interactiveCount, linkCount, paragraphCount, headings.length, preview);

  return {
    url: window.location.href,
    title: document.title || '',
    type: pageType,
    headings: headings,
    nav_links: navLinks,
    forms: forms,
    interactive_element_count: interactiveCount,
    main_content_preview: preview,
    word_count: wordCount
  };
})()`

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
		"script":     pageSummaryScript,
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
		h.processAPIValidationBodies()
		filter := h.apiValidationFilter(params.URLFilter, params.IgnoreEndpoints)
		result := h.apiContractAnalyze(filter)
		responseData := map[string]any{
			"status":                   "ok",
			"operation":                "analyze",
			"action":                   result.Action,
			"analyzed_at":              result.AnalyzedAt,
			"summary":                  result.Summary,
			"violations":               result.Violations,
			"tracked_endpoints":        result.TrackedEndpoints,
			"total_requests_analyzed":  result.TotalRequestsAnalyzed,
			"clean_endpoints":          result.CleanEndpoints,
			"possible_violation_types": result.PossibleViolationTypes,
		}
		if result.DataWindowStartedAt != "" {
			responseData["data_window_started_at"] = result.DataWindowStartedAt
		}
		if result.AppliedFilter != nil {
			responseData["applied_filter"] = result.AppliedFilter
		}
		if result.Hint != "" {
			responseData["hint"] = result.Hint
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API validation analyze", responseData)}

	case "report":
		h.processAPIValidationBodies()
		filter := h.apiValidationFilter(params.URLFilter, params.IgnoreEndpoints)
		result := h.apiContractReport(filter)
		responseData := map[string]any{
			"status":             "ok",
			"operation":          "report",
			"action":             result.Action,
			"analyzed_at":        result.AnalyzedAt,
			"endpoints":          result.Endpoints,
			"consistency_levels": result.ConsistencyLevels,
		}
		if result.AppliedFilter != nil {
			responseData["applied_filter"] = result.AppliedFilter
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API validation report", responseData)}

	case "clear":
		h.clearAPIValidationState()
		clearResult := map[string]any{
			"action":    "cleared",
			"status":    "ok",
			"operation": "clear",
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API validation cleared", clearResult)}

	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "operation parameter must be 'analyze', 'report', or 'clear'", "Use a valid value for 'operation'", withParam("operation"), withHint("analyze, report, or clear"))}
	}
}

func (h *ToolHandler) processAPIValidationBodies() {
	h.apiContractMu.Lock()
	defer h.apiContractMu.Unlock()

	if h.apiContractValidator == nil {
		h.apiContractValidator = analysis.NewAPIContractValidator()
	}

	bodies := h.capture.GetNetworkBodies()
	if h.apiContractOffset < 0 || h.apiContractOffset > len(bodies) {
		// Buffer can shrink due ring eviction; clamp to avoid replaying old data.
		h.apiContractOffset = len(bodies)
	}

	for _, body := range bodies[h.apiContractOffset:] {
		h.apiContractValidator.Validate(body)
	}
	h.apiContractOffset = len(bodies)
}

func (h *ToolHandler) apiValidationFilter(urlFilter string, ignore []string) analysis.APIContractFilter {
	return analysis.APIContractFilter{
		URLFilter:       urlFilter,
		IgnoreEndpoints: ignore,
	}
}

func (h *ToolHandler) apiContractAnalyze(filter analysis.APIContractFilter) analysis.APIContractAnalyzeResult {
	h.apiContractMu.Lock()
	validator := h.apiContractValidator
	h.apiContractMu.Unlock()
	if validator == nil {
		return analysis.APIContractAnalyzeResult{}
	}
	return validator.Analyze(filter)
}

func (h *ToolHandler) apiContractReport(filter analysis.APIContractFilter) analysis.APIContractReportResult {
	h.apiContractMu.Lock()
	validator := h.apiContractValidator
	h.apiContractMu.Unlock()
	if validator == nil {
		return analysis.APIContractReportResult{}
	}
	return validator.Report(filter)
}

func (h *ToolHandler) clearAPIValidationState() {
	h.apiContractMu.Lock()
	defer h.apiContractMu.Unlock()

	h.apiContractValidator = analysis.NewAPIContractValidator()
	h.apiContractOffset = len(h.capture.GetNetworkBodies())
}

// ============================================
// Link Health
// ============================================

// toolAnalyzeLinkHealth checks all links on the current page for health issues.
func (h *ToolHandler) toolAnalyzeLinkHealth(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID int `json:"tab_id,omitempty"`
	}
	_ = json.Unmarshal(args, &params) // best-effort: tab_id is optional

	// Generate correlation ID for tracking
	correlationID := fmt.Sprintf("link_health_%d_%d", time.Now().UnixNano(), randomInt63())

	// Create pending query for link health check
	query := queries.PendingQuery{
		Type:          "link_health",
		Params:        args,
		TabID:         params.TabID,
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
