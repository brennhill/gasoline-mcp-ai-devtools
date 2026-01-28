// queries.go — On-demand DOM and accessibility queries via the browser extension.
// Implements a request/response pattern: the MCP tool posts a query, the
// extension picks it up via polling, executes it in the page, and posts results.
// Design: Pending queries have a TTL and are garbage-collected. Accessibility
// audit results are cached with a configurable TTL to avoid redundant scans.
package main

import (
	crypto_rand "crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

// ============================================
// Polling Activity Log
// ============================================

// logPollingActivity adds an entry to the circular polling log buffer.
// Thread-safe: caller must hold c.mu lock.
func (c *Capture) logPollingActivity(entry PollingLogEntry) {
	c.pollingLog[c.pollingLogIndex] = entry
	c.pollingLogIndex = (c.pollingLogIndex + 1) % 50
}

// logHTTPDebugEntry adds an entry to the circular HTTP debug log buffer.
// Thread-safe: caller must hold c.mu lock.
// Does NOT print to stderr - caller should call printHTTPDebug() after unlocking.
func (c *Capture) logHTTPDebugEntry(entry HTTPDebugEntry) {
	c.httpDebugLog[c.httpDebugLogIndex] = entry
	c.httpDebugLogIndex = (c.httpDebugLogIndex + 1) % 50
}

// printHTTPDebug prints an HTTP debug entry to stderr.
// Must be called WITHOUT holding the lock to avoid deadlock.
func printHTTPDebug(entry HTTPDebugEntry) {
	fmt.Fprintf(os.Stderr, "[gasoline] HTTP %s %s | session=%s client=%s status=%d duration=%dms\n",
		entry.Method, entry.Endpoint, entry.SessionID, entry.ClientID, entry.ResponseStatus, entry.DurationMs)
	if entry.RequestBody != "" {
		fmt.Fprintf(os.Stderr, "[gasoline]   Request: %s\n", entry.RequestBody)
	}
	if entry.ResponseBody != "" {
		fmt.Fprintf(os.Stderr, "[gasoline]   Response: %s\n", entry.ResponseBody)
	}
	if entry.Error != "" {
		fmt.Fprintf(os.Stderr, "[gasoline]   Error: %s\n", entry.Error)
	}
}

// GetPollingLog returns the most recent polling activity (up to 50 entries).
// Returns entries in chronological order (oldest first).
func (c *Capture) GetPollingLog() []PollingLogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]PollingLogEntry, 0, 50)
	// Read from oldest to newest: start at current index (oldest) and wrap around
	for i := 0; i < 50; i++ {
		idx := (c.pollingLogIndex + i) % 50
		entry := c.pollingLog[idx]
		// Skip empty entries (buffer not yet full)
		if !entry.Timestamp.IsZero() {
			result = append(result, entry)
		}
	}
	return result
}

// GetHTTPDebugLog returns the most recent HTTP debug entries (up to 50 entries).
// Returns entries in chronological order (oldest first).
func (c *Capture) GetHTTPDebugLog() []HTTPDebugEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]HTTPDebugEntry, 0, 50)
	// Read from oldest to newest: start at current index (oldest) and wrap around
	for i := 0; i < 50; i++ {
		idx := (c.httpDebugLogIndex + i) % 50
		entry := c.httpDebugLog[idx]
		// Skip empty entries (buffer not yet full)
		if !entry.Timestamp.IsZero() {
			result = append(result, entry)
		}
	}
	return result
}

// ============================================
// Pending Queries
// ============================================

// CreatePendingQuery creates a pending query and returns its ID
func (c *Capture) CreatePendingQuery(query PendingQuery) string {
	return c.CreatePendingQueryWithClient(query, c.queryTimeout, "")
}

// CreatePendingQueryWithClient creates a pending query with client ID for isolation
func (c *Capture) CreatePendingQueryWithClient(query PendingQuery, timeout time.Duration, clientID string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Enforce max pending queries
	if len(c.pendingQueries) >= maxPendingQueries {
		// Drop oldest
		newQueries := make([]pendingQueryEntry, len(c.pendingQueries)-1)
		copy(newQueries, c.pendingQueries[1:])
		c.pendingQueries = newQueries
	}

	c.queryIDCounter++
	id := fmt.Sprintf("q-%d", c.queryIDCounter)

	entry := pendingQueryEntry{
		query: PendingQueryResponse{
			ID:            id,
			Type:          query.Type,
			Params:        query.Params,
			TabID:         query.TabID,
			CorrelationID: query.CorrelationID, // Pass through correlation_id for async command tracking
		},
		expires:  time.Now().Add(timeout),
		clientID: clientID,
	}

	c.pendingQueries = append(c.pendingQueries, entry)

	return id
}

// CreatePendingQueryWithTimeout creates a pending query with a custom timeout (legacy, no client isolation)
func (c *Capture) CreatePendingQueryWithTimeout(query PendingQuery, timeout time.Duration) string {
	return c.CreatePendingQueryWithClient(query, timeout, "")
}

// cleanExpiredQueries removes expired pending queries and stale results (must hold lock)
func (c *Capture) cleanExpiredQueries() {
	now := time.Now()
	// Use new slice to allow GC of expired entries (avoids [:0] backing-array pinning)
	remaining := make([]pendingQueryEntry, 0, len(c.pendingQueries))
	for _, pq := range c.pendingQueries {
		if pq.expires.After(now) {
			remaining = append(remaining, pq)
		}
	}
	c.pendingQueries = remaining

	// Sweep stale query results (stored but never consumed, e.g. client timed out)
	for id, entry := range c.queryResults {
		if !entry.createdAt.IsZero() && now.Sub(entry.createdAt) > 60*time.Second {
			delete(c.queryResults, id)
		}
	}
}

// startQueryCleanup starts a periodic goroutine that cleans expired pending queries
// every 5 seconds. Returns a stop function to terminate the goroutine.
// Replaces per-query goroutines with a single consolidated cleanup ticker.
func (c *Capture) startQueryCleanup() func() {
	stop := make(chan struct{})
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.mu.Lock()
				if len(c.pendingQueries) > 0 {
					c.cleanExpiredQueries()
					c.queryCond.Broadcast()
				}
				c.mu.Unlock()
			case <-stop:
				return
			}
		}
	}()
	return func() { close(stop) }
}

// GetPendingQueries returns all pending queries
func (c *Capture) GetPendingQueries() []PendingQueryResponse {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cleanExpiredQueries()

	result := make([]PendingQueryResponse, 0, len(c.pendingQueries))
	for _, pq := range c.pendingQueries {
		result = append(result, pq.query)
	}
	return result
}

// SetQueryResult stores the result for a pending query
func (c *Capture) SetQueryResult(id string, result json.RawMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find the pending query to get its client ID
	var clientID string
	for _, pq := range c.pendingQueries {
		if pq.query.ID == id {
			clientID = pq.clientID
			break
		}
	}

	c.queryResults[id] = queryResultEntry{
		result:    result,
		clientID:  clientID,
		createdAt: time.Now(),
	}

	// Remove from pending
	remaining := make([]pendingQueryEntry, 0, len(c.pendingQueries))
	for _, pq := range c.pendingQueries {
		if pq.query.ID != id {
			remaining = append(remaining, pq)
		}
	}
	c.pendingQueries = remaining

	// Wake up waiters
	c.queryCond.Broadcast()
}

// GetQueryResult retrieves the result for a query and deletes it from storage.
// If clientID is provided, only returns the result if it belongs to that client.
// If clientID is not provided (legacy), only returns results without client isolation.
func (c *Capture) GetQueryResult(id string, clientID string) (json.RawMessage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, found := c.queryResults[id]
	if found {
		// Strict isolation: verify client ownership
		if clientID == "" {
			// No client ID provided: only return results without isolation (legacy results)
			if entry.clientID != "" {
				return nil, false
			}
		} else {
			// Client ID provided: only return results for that exact client
			if entry.clientID != clientID {
				return nil, false
			}
		}
		delete(c.queryResults, id)
	}
	return entry.result, found
}

// GetLastPollAt returns when the extension last polled /pending-queries
func (c *Capture) GetLastPollAt() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastPollAt
}

// WaitForResult blocks until a result is available or timeout, then deletes it.
// If clientID is provided, only returns the result if it belongs to that client.
// Uses a cancel channel to prevent goroutine leaks on timeout.
func (c *Capture) WaitForResult(id string, timeout time.Duration, clientID string) (json.RawMessage, error) {
	resultChan := make(chan json.RawMessage, 1)
	errChan := make(chan error, 1)
	var cancelled atomic.Bool

	// Goroutine to wait for the result
	go func() {
		done := make(chan struct{})
		defer close(done)

		// Ticker broadcasts periodically to recheck
		go func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					c.queryCond.Broadcast()
				case <-done:
					return
				}
			}
		}()

		c.mu.Lock()
		defer c.mu.Unlock()

		for {
			// Check if cancelled before inspecting results
			if cancelled.Load() {
				return
			}
			if entry, found := c.queryResults[id]; found {
				// Strict isolation: verify client ownership
				if clientID == "" {
					// No client ID provided: only return results without isolation (legacy results)
					if entry.clientID != "" {
						errChan <- fmt.Errorf("permission denied for result %s", id)
						return
					}
				} else {
					// Client ID provided: only return results for that exact client
					if entry.clientID != clientID {
						errChan <- fmt.Errorf("permission denied for result %s", id)
						return
					}
				}
				delete(c.queryResults, id)
				resultChan <- entry.result
				return
			}
			c.queryCond.Wait()
			// Check if cancelled after waking from Wait
			if cancelled.Load() {
				return
			}
		}
	}()

	// Select with hard timeout - never hangs beyond timeout duration
	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errChan:
		return nil, err
	case <-time.After(timeout):
		// Signal the inner goroutine to exit
		cancelled.Store(true)
		// Broadcast to wake the goroutine so it can observe the cancellation
		c.queryCond.Broadcast()
		return nil, fmt.Errorf("timeout waiting for result %s", id)
	}
}

// WaitForResultLegacy is the legacy version without client isolation
func (c *Capture) WaitForResultLegacy(id string, timeout time.Duration) (json.RawMessage, error) {
	return c.WaitForResult(id, timeout, "")
}

// SetQueryTimeout sets the default timeout for on-demand queries
func (c *Capture) SetQueryTimeout(timeout time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queryTimeout = timeout
}

func (c *Capture) HandlePendingQueries(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	sessionID := r.Header.Get("X-Gasoline-Session")
	pilotHeader := r.Header.Get("X-Gasoline-Pilot")
	clientID := r.Header.Get("X-Gasoline-Client")

	// Collect all headers for debug logging (redact auth)
	headers := make(map[string]string)
	for name, values := range r.Header {
		if strings.Contains(strings.ToLower(name), "auth") || strings.Contains(strings.ToLower(name), "token") {
			headers[name] = "[REDACTED]"
		} else if len(values) > 0 {
			headers[name] = values[0]
		}
	}

	// Track when extension last polled and detect session changes (reloads)
	c.mu.Lock()
	c.lastPollAt = startTime

	// Update pilot state from header only if we don't have recent POST-based settings
	// See docs/plugin-server-communications.md for protocol details
	settingsAge := startTime.Sub(c.pilotUpdatedAt)
	if settingsAge > 10*time.Second {
		// No recent POST /settings, use header for backward compatibility
		c.pilotEnabled = pilotHeader == "1"
		c.pilotUpdatedAt = startTime
	}
	// Otherwise, keep cached value from POST /settings

	if sessionID != "" && sessionID != c.extensionSession {
		if c.extensionSession != "" {
			// Session changed - extension was reloaded
			fmt.Fprintf(os.Stderr, "[gasoline] Extension reloaded: %s -> %s\n", c.extensionSession, sessionID)
		} else {
			// First connection
			fmt.Fprintf(os.Stderr, "[gasoline] Extension connected: %s\n", sessionID)
		}
		c.extensionSession = sessionID
		c.sessionChangedAt = startTime
	}

	// Get queries before unlocking (while still holding lock)
	// Note: Cannot call GetPendingQueries() here since we already hold the lock
	c.cleanExpiredQueries()
	queries := make([]PendingQueryResponse, 0, len(c.pendingQueries))
	for _, pq := range c.pendingQueries {
		queries = append(queries, pq.query)
	}

	// Log polling activity to circular buffer
	c.logPollingActivity(PollingLogEntry{
		Timestamp:   startTime,
		Endpoint:    "pending-queries",
		Method:      "GET",
		SessionID:   sessionID,
		PilotHeader: pilotHeader,
		QueryCount:  len(queries),
	})

	resp := map[string]interface{}{
		"queries": queries,
	}
	responseJSON, _ := json.Marshal(resp)
	responsePreview := string(responseJSON)
	if len(responsePreview) > 1000 {
		responsePreview = responsePreview[:1000] + "..."
	}

	// Log HTTP debug entry
	duration := time.Since(startTime)
	debugEntry := HTTPDebugEntry{
		Timestamp:      startTime,
		Endpoint:       "/pending-queries",
		Method:         "GET",
		SessionID:      sessionID,
		ClientID:       clientID,
		Headers:        headers,
		ResponseStatus: http.StatusOK,
		ResponseBody:   responsePreview,
		DurationMs:     duration.Milliseconds(),
	}
	c.logHTTPDebugEntry(debugEntry)

	c.mu.Unlock()

	// Print debug log after unlocking to avoid deadlock
	printHTTPDebug(debugEntry)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// HandleDOMResult handles POST /dom-result
func (c *Capture) HandleDOMResult(w http.ResponseWriter, r *http.Request) {
	c.handleQueryResult(w, r)
}

// HandleA11yResult handles POST /a11y-result
func (c *Capture) HandleA11yResult(w http.ResponseWriter, r *http.Request) {
	c.handleQueryResult(w, r)
}

// HandleStateResult handles POST /state-result (pilot state management)
func (c *Capture) HandleStateResult(w http.ResponseWriter, r *http.Request) {
	c.handleQueryResult(w, r)
}

// HandleExecuteResult handles POST /execute-result
func (c *Capture) HandleExecuteResult(w http.ResponseWriter, r *http.Request) {
	c.handleQueryResult(w, r)
}

// HandleHighlightResult handles POST /highlight-result (pilot highlight commands)
func (c *Capture) HandleHighlightResult(w http.ResponseWriter, r *http.Request) {
	c.handleQueryResult(w, r)
}

// handleQueryResult handles a query result POST (shared between DOM and A11y)
func (c *Capture) handleQueryResult(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	var payload struct {
		ID            string          `json:"id"`              // Legacy sync query ID
		CorrelationID string          `json:"correlation_id"`  // Async command correlation ID
		Status        string          `json:"status"`          // "pending", "complete", "timeout"
		Result        json.RawMessage `json:"result"`
		Error         string          `json:"error,omitempty"` // Error message for failed commands
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// ASYNC COMMAND PATH: If correlation_id is present, store in async tracking
	if payload.CorrelationID != "" {
		c.SetCommandResult(CommandResult{
			CorrelationID: payload.CorrelationID,
			Status:        payload.Status,
			Result:        payload.Result,
			Error:         payload.Error,
			CreatedAt:     time.Now(), // Will be overridden if already exists
		})
		w.WriteHeader(http.StatusOK)
		return
	}

	// LEGACY SYNC PATH: Use existing query result mechanism
	if payload.ID != "" {
		// Atomically check existence and store result (single lock, no TOCTOU gap)
		if !c.setQueryResultIfExists(payload.ID, payload.Result) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	// Neither ID nor correlation_id provided
	w.WriteHeader(http.StatusBadRequest)
}

// setQueryResultIfExists atomically checks if a query ID exists (pending or
// already answered) and stores the result. Returns false if not found.
func (c *Capture) setQueryResultIfExists(id string, result json.RawMessage) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clean expired queries before checking so stale entries are rejected
	c.cleanExpiredQueries()

	// Check pending queries
	found := false
	var clientID string
	for _, pq := range c.pendingQueries {
		if pq.query.ID == id {
			found = true
			clientID = pq.clientID
			break
		}
	}
	if _, exists := c.queryResults[id]; exists {
		found = true
	}
	if !found {
		return false
	}

	// Store result
	c.queryResults[id] = queryResultEntry{
		result:    result,
		clientID:  clientID,
		createdAt: time.Now(),
	}

	// Remove from pending (new slice avoids GC pinning)
	remaining := make([]pendingQueryEntry, 0, len(c.pendingQueries))
	for _, pq := range c.pendingQueries {
		if pq.query.ID != id {
			remaining = append(remaining, pq)
		}
	}
	c.pendingQueries = remaining

	c.queryCond.Broadcast()
	return true
}

// HandleEnhancedActions handles POST /enhanced-actions

// DOM query constants — must match extension/lib/constants.js values
const (
	domQueryMaxElements = 50
	domQueryMaxDepth    = 5
	domQueryMaxText     = 500
)

func (h *ToolHandler) toolQueryDOM(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		Selector string `json:"selector"`
		TabID    int    `json:"tab_id"`
	}
	_ = json.Unmarshal(args, &arguments) // Optional args - zero values are acceptable defaults

	params, _ := json.Marshal(map[string]string{"selector": arguments.Selector})
	id := h.capture.CreatePendingQueryWithClient(PendingQuery{
		Type:   "dom",
		Params: params,
		TabID:  arguments.TabID,
	}, h.capture.queryTimeout, req.ClientID)

	result, err := h.capture.WaitForResult(id, h.capture.queryTimeout, req.ClientID)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrExtTimeout, "Timeout waiting for DOM query result", "Browser extension didn't respond — wait a moment and retry", withHint("Check that the browser extension is connected and a page is focused"))}
	}

	// Parse extension response to restructure for LLM consumption
	var extResult struct {
		URL           string                   `json:"url"`
		Title         string                   `json:"title"`
		MatchCount    int                      `json:"matchCount"`    // TODO v6.0: migrate to match_count
		ReturnedCount int                      `json:"returnedCount"` // TODO v6.0: migrate to returned_count
		Matches       []map[string]interface{} `json:"matches"`
	}
	_ = json.Unmarshal(result, &extResult)

	// Process matches: add textTruncated, rename boundingBox → bboxPixels
	for _, match := range extResult.Matches {
		// Add textTruncated boolean based on text length vs max
		if text, ok := match["text"].(string); ok {
			match["textTruncated"] = len(text) >= domQueryMaxText
		} else {
			match["textTruncated"] = false
		}

		// Rename boundingBox → bboxPixels (unit suffix for clarity)
		if bbox, ok := match["boundingBox"]; ok {
			match["bboxPixels"] = bbox
			delete(match, "boundingBox")
		}
	}

	// Build structured response with metadata
	data := map[string]interface{}{
		"url":                 extResult.URL,
		"pageTitle":           extResult.Title,
		"selector":            arguments.Selector,
		"totalMatchCount":     extResult.MatchCount,
		"returnedMatchCount":  extResult.ReturnedCount,
		"maxElementsReturned": domQueryMaxElements,
		"maxDepthQueried":     domQueryMaxDepth,
		"maxTextLength":       domQueryMaxText,
		"matches":             extResult.Matches,
	}

	// Add helpful hint when no matches found
	if len(extResult.Matches) == 0 {
		data["hint"] = fmt.Sprintf("No elements matched selector %q. Verify the selector is correct and the page has loaded. Try a broader selector like 'div' or '*' to explore the DOM.", arguments.Selector)
	}

	summary := fmt.Sprintf("DOM query %q: %d match(es)", arguments.Selector, extResult.ReturnedCount)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, data)}
}

func (h *ToolHandler) toolGetPageInfo(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	id := h.capture.CreatePendingQueryWithClient(PendingQuery{
		Type:   "page_info",
		Params: json.RawMessage(`{}`),
	}, h.capture.queryTimeout, req.ClientID)

	result, err := h.capture.WaitForResult(id, h.capture.queryTimeout, req.ClientID)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrExtTimeout, "Timeout waiting for page info", "Browser extension didn't respond — wait a moment and retry", withHint("Check that the browser extension is connected and a page is focused"))}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Page info", json.RawMessage(result))}
}

// toolGetTabs requests the list of all open browser tabs from the extension.
// This is used by AI Web Pilot to enable tab targeting - allowing tools to
// operate on specific tabs rather than just the active tab.
func (h *ToolHandler) toolGetTabs(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	id := h.capture.CreatePendingQueryWithClient(PendingQuery{
		Type:   "tabs",
		Params: json.RawMessage(`{}`),
	}, h.capture.queryTimeout, req.ClientID)

	result, err := h.capture.WaitForResult(id, h.capture.queryTimeout, req.ClientID)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrExtTimeout, "Timeout waiting for tabs list", "Browser extension didn't respond — wait a moment and retry", withHint("Check that the browser extension is connected and a page is focused"))}
	}

	// Parse result to count tabs for summary
	var tabs []interface{}
	_ = json.Unmarshal(result, &tabs)
	summary := fmt.Sprintf("%d browser tab(s)", len(tabs))
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, json.RawMessage(result))}
}

func (h *ToolHandler) toolRunA11yAudit(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		Scope        string   `json:"scope"`
		Tags         []string `json:"tags"`
		ForceRefresh bool     `json:"force_refresh"`
	}
	_ = json.Unmarshal(args, &arguments) // Optional args - zero values are acceptable defaults

	cacheKey := h.capture.a11yCacheKey(arguments.Scope, arguments.Tags)

	// Check cache (unless force_refresh)
	if !arguments.ForceRefresh {
		if cached := h.capture.getA11yCacheEntry(cacheKey); cached != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Accessibility audit", json.RawMessage(cached))}
		}

		// Check if there's an inflight request for this key (concurrent dedup)
		if inflight := h.capture.getOrCreateInflight(cacheKey); inflight != nil {
			// Wait for the inflight request to complete
			<-inflight.done
			if inflight.err != nil {
				return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrExtTimeout, "Timeout waiting for accessibility audit result", "Browser extension didn't respond — wait a moment and retry", withHint("Check that the browser extension is connected and a page is focused"))}
			}
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Accessibility audit", json.RawMessage(inflight.result))}
		}
	} else {
		// force_refresh: remove existing cache entry
		h.capture.removeA11yCacheEntry(cacheKey)
		// Register inflight
		h.capture.getOrCreateInflight(cacheKey)
	}

	params := map[string]interface{}{}
	if arguments.Scope != "" {
		params["scope"] = arguments.Scope
	}
	if arguments.Tags != nil {
		params["tags"] = arguments.Tags
	}
	paramsJSON, _ := json.Marshal(params)

	id := h.capture.CreatePendingQueryWithClient(PendingQuery{
		Type:   "a11y",
		Params: paramsJSON,
	}, h.capture.queryTimeout, req.ClientID)

	result, err := h.capture.WaitForResult(id, h.capture.queryTimeout, req.ClientID)
	if err != nil {
		// Don't cache errors — complete inflight with error
		h.capture.completeInflight(cacheKey, nil, err)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrExtTimeout, "Timeout waiting for accessibility audit result", "Browser extension didn't respond — wait a moment and retry", withHint("Check that the browser extension is connected and a page is focused"))}
	}

	// Cache the successful result
	h.capture.setA11yCacheEntry(cacheKey, result)
	h.capture.completeInflight(cacheKey, result, nil)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Accessibility audit", json.RawMessage(result))}
}

// ============================================
// A11y Audit Cache
// ============================================

// a11yCacheKey generates a cache key from scope and tags (tags sorted for normalization)
func (c *Capture) a11yCacheKey(scope string, tags []string) string {
	sortedTags := make([]string, len(tags))
	copy(sortedTags, tags)
	sort.Strings(sortedTags)
	return scope + "|" + strings.Join(sortedTags, ",")
}

// getA11yCacheEntry returns cached result if valid, nil otherwise
func (c *Capture) getA11yCacheEntry(key string) json.RawMessage {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.a11y.cache[key]
	if !exists {
		return nil
	}
	if time.Since(entry.createdAt) > a11yCacheTTL {
		return nil
	}
	if c.a11y.lastURL != "" && entry.url != "" && entry.url != c.a11y.lastURL {
		return nil
	}
	return entry.result
}

// setA11yCacheEntry stores a result in the cache with eviction
func (c *Capture) setA11yCacheEntry(key string, result json.RawMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.a11y.cache[key]; !exists && len(c.a11y.cache) >= maxA11yCacheEntries {
		if len(c.a11y.cacheOrder) > 0 {
			oldest := c.a11y.cacheOrder[0]
			newOrder := make([]string, len(c.a11y.cacheOrder)-1)
			copy(newOrder, c.a11y.cacheOrder[1:])
			c.a11y.cacheOrder = newOrder
			delete(c.a11y.cache, oldest)
		}
	}

	c.a11y.cache[key] = &a11yCacheEntry{
		result:    result,
		createdAt: time.Now(),
		url:       c.a11y.lastURL,
	}

	newOrder := make([]string, 0, len(c.a11y.cacheOrder)+1)
	for _, k := range c.a11y.cacheOrder {
		if k != key {
			newOrder = append(newOrder, k)
		}
	}
	newOrder = append(newOrder, key)
	c.a11y.cacheOrder = newOrder
}

// removeA11yCacheEntry removes a specific cache entry
func (c *Capture) removeA11yCacheEntry(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.a11y.cache, key)
	newOrder := make([]string, 0, len(c.a11y.cacheOrder))
	for _, k := range c.a11y.cacheOrder {
		if k != key {
			newOrder = append(newOrder, k)
		}
	}
	c.a11y.cacheOrder = newOrder
}

// getOrCreateInflight returns an existing inflight entry to wait on, or nil if this caller should proceed.
func (c *Capture) getOrCreateInflight(key string) *a11yInflightEntry {
	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.a11y.inflight[key]; ok {
		return existing
	}
	c.a11y.inflight[key] = &a11yInflightEntry{
		done: make(chan struct{}),
	}
	return nil
}

// completeInflight signals waiters and removes the inflight entry
func (c *Capture) completeInflight(key string, result json.RawMessage, err error) {
	c.mu.Lock()
	entry, exists := c.a11y.inflight[key]
	if exists {
		entry.result = result
		entry.err = err
		delete(c.a11y.inflight, key)
	}
	c.mu.Unlock()

	if exists {
		close(entry.done)
	}
}

// ExpireA11yCache forces all cache entries to expire (for testing)
func (c *Capture) ExpireA11yCache() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, entry := range c.a11y.cache {
		entry.createdAt = time.Now().Add(-a11yCacheTTL - time.Second)
		c.a11y.cache[key] = entry
	}
}

// GetA11yCacheSize returns the number of entries in the a11y cache
func (c *Capture) GetA11yCacheSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.a11y.cache)
}

// SetLastKnownURL updates the last known page URL for navigation detection.
func (c *Capture) SetLastKnownURL(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.a11y.lastURL != "" && url != c.a11y.lastURL {
		c.a11y.cache = make(map[string]*a11yCacheEntry)
		c.a11y.cacheOrder = make([]string, 0)
	}
	c.a11y.lastURL = url
}

// ============================================
// Async Command Result Management
// ============================================

// generateCorrelationID creates a unique correlation ID for async command tracking.
// Format: corr-<timestamp-millis>-<random-hex>
// Uses crypto/rand for the random component to ensure uniqueness.
func generateCorrelationID() string {
	b := make([]byte, 8)
	crypto_rand.Read(b)
	return fmt.Sprintf("corr-%d-%x", time.Now().UnixMilli(), b)
}

// SetCommandResult stores or updates an async command result.
// Thread-safe with separate lock to avoid blocking main capture operations.
func (c *Capture) SetCommandResult(result CommandResult) {
	c.resultsMu.Lock()
	defer c.resultsMu.Unlock()

	if result.Status == "complete" || result.Status == "timeout" {
		result.CompletedAt = time.Now()
		c.completedResults[result.CorrelationID] = &result
	} else if result.Status == "pending" {
		// Extension acknowledged, command still running - keep in completedResults with pending status
		if _, exists := c.completedResults[result.CorrelationID]; !exists {
			result.CreatedAt = time.Now()
			c.completedResults[result.CorrelationID] = &result
		}
	}
}

// GetCommandResult retrieves an async command result by correlation ID.
// Returns nil if not found. Thread-safe.
func (c *Capture) GetCommandResult(correlationID string) *CommandResult {
	c.resultsMu.RLock()
	defer c.resultsMu.RUnlock()

	if result, found := c.completedResults[correlationID]; found {
		// Make a copy to avoid holding lock during JSON marshaling
		resultCopy := *result
		return &resultCopy
	}
	return nil
}

// GetPendingCommands returns all pending, completed, and failed commands.
// Thread-safe - returns copies to avoid lock contention.
func (c *Capture) GetPendingCommands() (pending, completed, failed []*CommandResult) {
	c.resultsMu.RLock()
	defer c.resultsMu.RUnlock()

	pending = make([]*CommandResult, 0)
	completed = make([]*CommandResult, 0)

	for _, result := range c.completedResults {
		resultCopy := *result
		if result.Status == "pending" {
			pending = append(pending, &resultCopy)
		} else if result.Status == "complete" || result.Status == "timeout" {
			completed = append(completed, &resultCopy)
		}
	}

	// Copy failed commands
	failed = make([]*CommandResult, len(c.failedCommands))
	for i, cmd := range c.failedCommands {
		cmdCopy := *cmd
		failed[i] = &cmdCopy
	}

	return pending, completed, failed
}

// GetFailedCommands returns recent failed/expired commands.
// Thread-safe - returns copies.
func (c *Capture) GetFailedCommands() []*CommandResult {
	c.resultsMu.RLock()
	defer c.resultsMu.RUnlock()

	failed := make([]*CommandResult, len(c.failedCommands))
	for i, cmd := range c.failedCommands {
		cmdCopy := *cmd
		failed[i] = &cmdCopy
	}
	return failed
}

// startResultCleanup starts a goroutine that expires old command results after 60s.
// Should be called once during server initialization.
// Returns a stop function that terminates the cleanup goroutine.
func (c *Capture) startResultCleanup() func() {
	stop := make(chan struct{})
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.resultsMu.Lock()
				now := time.Now()
				for correlationID, result := range c.completedResults {
					// Skip pending commands - they haven't completed yet
					if result.Status == "pending" {
						continue
					}

					age := now.Sub(result.CompletedAt)
					if age > 60*time.Second {
						// Move to failed_commands if never retrieved
						c.failedCommands = append(c.failedCommands, &CommandResult{
							CorrelationID: correlationID,
							Status:        "expired",
							Error:         "Result expired after 60s (LLM never retrieved)",
							CompletedAt:   result.CompletedAt,
							CreatedAt:     result.CreatedAt,
						})

						// Trim failedCommands to max 100 entries (circular buffer)
						if len(c.failedCommands) > 100 {
							c.failedCommands = c.failedCommands[len(c.failedCommands)-100:]
						}

						delete(c.completedResults, correlationID)

						// Log for observability
						fmt.Fprintf(os.Stderr, "[gasoline] Expired unretrieved result: correlation_id=%s\n", correlationID)
					}
				}
				c.resultsMu.Unlock()
			case <-stop:
				return
			}
		}
	}()
	return func() { close(stop) }
}
