// queries.go — On-demand DOM and accessibility queries via the browser extension.
// Implements a request/response pattern: the MCP tool posts a query, the
// extension picks it up via polling, executes it in the page, and posts results.
// Design: Pending queries have a TTL and are garbage-collected. Accessibility
// audit results are cached with a configurable TTL to avoid redundant scans.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// ============================================
// Pending Queries
// ============================================

// CreatePendingQuery creates a pending query and returns its ID
func (v *Capture) CreatePendingQuery(query PendingQuery) string {
	return v.CreatePendingQueryWithTimeout(query, v.queryTimeout)
}

// CreatePendingQueryWithTimeout creates a pending query with a custom timeout
func (v *Capture) CreatePendingQueryWithTimeout(query PendingQuery, timeout time.Duration) string {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Enforce max pending queries
	if len(v.pendingQueries) >= maxPendingQueries {
		// Drop oldest
		v.pendingQueries = v.pendingQueries[1:]
	}

	v.queryIDCounter++
	id := fmt.Sprintf("q-%d", v.queryIDCounter)

	entry := pendingQueryEntry{
		query: PendingQueryResponse{
			ID:     id,
			Type:   query.Type,
			Params: query.Params,
		},
		expires: time.Now().Add(timeout),
	}

	v.pendingQueries = append(v.pendingQueries, entry)

	// Schedule cleanup
	go func() {
		time.Sleep(timeout)
		v.mu.Lock()
		defer v.mu.Unlock()
		v.cleanExpiredQueries()
		v.queryCond.Broadcast()
	}()

	return id
}

// cleanExpiredQueries removes expired pending queries (must hold lock)
func (v *Capture) cleanExpiredQueries() {
	now := time.Now()
	remaining := v.pendingQueries[:0]
	for _, pq := range v.pendingQueries {
		if pq.expires.After(now) {
			remaining = append(remaining, pq)
		}
	}
	v.pendingQueries = remaining
}

// GetPendingQueries returns all pending queries
func (v *Capture) GetPendingQueries() []PendingQueryResponse {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.cleanExpiredQueries()

	result := make([]PendingQueryResponse, 0, len(v.pendingQueries))
	for _, pq := range v.pendingQueries {
		result = append(result, pq.query)
	}
	return result
}

// SetQueryResult stores the result for a pending query
func (v *Capture) SetQueryResult(id string, result json.RawMessage) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.queryResults[id] = result

	// Remove from pending
	remaining := v.pendingQueries[:0]
	for _, pq := range v.pendingQueries {
		if pq.query.ID != id {
			remaining = append(remaining, pq)
		}
	}
	v.pendingQueries = remaining

	// Wake up waiters
	v.queryCond.Broadcast()
}

// GetQueryResult retrieves the result for a query and deletes it from storage
func (v *Capture) GetQueryResult(id string) (json.RawMessage, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	result, found := v.queryResults[id]
	if found {
		delete(v.queryResults, id)
	}
	return result, found
}

// GetLastPollAt returns when the extension last polled /pending-queries
func (v *Capture) GetLastPollAt() time.Time {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.lastPollAt
}

// WaitForResult blocks until a result is available or timeout, then deletes it
func (v *Capture) WaitForResult(id string, timeout time.Duration) (json.RawMessage, error) {
	deadline := time.Now().Add(timeout)

	v.mu.Lock()
	defer v.mu.Unlock()

	for {
		if result, found := v.queryResults[id]; found {
			delete(v.queryResults, id)
			return result, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for result %s", id)
		}
		// Wait with a short timeout to recheck
		go func() {
			time.Sleep(10 * time.Millisecond)
			v.queryCond.Broadcast()
		}()
		v.queryCond.Wait()
	}
}

// SetQueryTimeout sets the default timeout for on-demand queries
func (v *Capture) SetQueryTimeout(timeout time.Duration) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.queryTimeout = timeout
}

func (v *Capture) HandlePendingQueries(w http.ResponseWriter, r *http.Request) {
	// Track when extension last polled
	v.mu.Lock()
	v.lastPollAt = time.Now()
	v.mu.Unlock()

	queries := v.GetPendingQueries()
	if queries == nil {
		queries = make([]PendingQueryResponse, 0)
	}

	resp := map[string]interface{}{
		"queries": queries,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// HandleDOMResult handles POST /dom-result
func (v *Capture) HandleDOMResult(w http.ResponseWriter, r *http.Request) {
	v.handleQueryResult(w, r)
}

// HandleA11yResult handles POST /a11y-result
func (v *Capture) HandleA11yResult(w http.ResponseWriter, r *http.Request) {
	v.handleQueryResult(w, r)
}

// HandleStateResult handles POST /state-result (pilot state management)
func (v *Capture) HandleStateResult(w http.ResponseWriter, r *http.Request) {
	v.handleQueryResult(w, r)
}

// HandleExecuteResult handles POST /execute-result
func (v *Capture) HandleExecuteResult(w http.ResponseWriter, r *http.Request) {
	v.handleQueryResult(w, r)
}

// handleQueryResult handles a query result POST (shared between DOM and A11y)
func (v *Capture) handleQueryResult(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	var payload struct {
		ID     string          `json:"id"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Check if this query ID exists (in pending or already answered)
	v.mu.RLock()
	found := false
	for _, pq := range v.pendingQueries {
		if pq.query.ID == payload.ID {
			found = true
			break
		}
	}
	if _, exists := v.queryResults[payload.ID]; exists {
		found = true
	}
	v.mu.RUnlock()

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	v.SetQueryResult(payload.ID, payload.Result)
	w.WriteHeader(http.StatusOK)
}

// HandleEnhancedActions handles POST /enhanced-actions

func (h *ToolHandler) toolQueryDOM(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		Selector string `json:"selector"`
	}
	_ = json.Unmarshal(args, &arguments) // Optional args - zero values are acceptable defaults

	params, _ := json.Marshal(map[string]string{"selector": arguments.Selector})
	id := h.capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: params,
	})

	result, err := h.capture.WaitForResult(id, h.capture.queryTimeout)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse("Timeout waiting for DOM query result")}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(result))}
}

func (h *ToolHandler) toolGetPageInfo(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	id := h.capture.CreatePendingQuery(PendingQuery{
		Type:   "page_info",
		Params: json.RawMessage(`{}`),
	})

	result, err := h.capture.WaitForResult(id, h.capture.queryTimeout)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse("Timeout waiting for page info")}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(result))}
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
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(cached))}
		}

		// Check if there's an inflight request for this key (concurrent dedup)
		if inflight := h.capture.getOrCreateInflight(cacheKey); inflight != nil {
			// Wait for the inflight request to complete
			<-inflight.done
			if inflight.err != nil {
				return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse("Timeout waiting for accessibility audit result")}
			}
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(inflight.result))}
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

	id := h.capture.CreatePendingQuery(PendingQuery{
		Type:   "a11y",
		Params: paramsJSON,
	})

	result, err := h.capture.WaitForResult(id, h.capture.queryTimeout)
	if err != nil {
		// Don't cache errors — complete inflight with error
		h.capture.completeInflight(cacheKey, nil, err)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse("Timeout waiting for accessibility audit result")}
	}

	// Cache the successful result
	h.capture.setA11yCacheEntry(cacheKey, result)
	h.capture.completeInflight(cacheKey, result, nil)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(result))}
}

// ============================================
// A11y Audit Cache
// ============================================

// a11yCacheKey generates a cache key from scope and tags (tags sorted for normalization)
func (v *Capture) a11yCacheKey(scope string, tags []string) string {
	sortedTags := make([]string, len(tags))
	copy(sortedTags, tags)
	sort.Strings(sortedTags)
	return scope + "|" + strings.Join(sortedTags, ",")
}

// getA11yCacheEntry returns cached result if valid, nil otherwise
func (v *Capture) getA11yCacheEntry(key string) json.RawMessage {
	v.mu.RLock()
	defer v.mu.RUnlock()

	entry, exists := v.a11y.cache[key]
	if !exists {
		return nil
	}
	if time.Since(entry.createdAt) > a11yCacheTTL {
		return nil
	}
	if v.a11y.lastURL != "" && entry.url != "" && entry.url != v.a11y.lastURL {
		return nil
	}
	return entry.result
}

// setA11yCacheEntry stores a result in the cache with eviction
func (v *Capture) setA11yCacheEntry(key string, result json.RawMessage) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if _, exists := v.a11y.cache[key]; !exists && len(v.a11y.cache) >= maxA11yCacheEntries {
		if len(v.a11y.cacheOrder) > 0 {
			oldest := v.a11y.cacheOrder[0]
			v.a11y.cacheOrder = v.a11y.cacheOrder[1:]
			delete(v.a11y.cache, oldest)
		}
	}

	v.a11y.cache[key] = &a11yCacheEntry{
		result:    result,
		createdAt: time.Now(),
		url:       v.a11y.lastURL,
	}

	newOrder := make([]string, 0, len(v.a11y.cacheOrder)+1)
	for _, k := range v.a11y.cacheOrder {
		if k != key {
			newOrder = append(newOrder, k)
		}
	}
	newOrder = append(newOrder, key)
	v.a11y.cacheOrder = newOrder
}

// removeA11yCacheEntry removes a specific cache entry
func (v *Capture) removeA11yCacheEntry(key string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	delete(v.a11y.cache, key)
	newOrder := make([]string, 0, len(v.a11y.cacheOrder))
	for _, k := range v.a11y.cacheOrder {
		if k != key {
			newOrder = append(newOrder, k)
		}
	}
	v.a11y.cacheOrder = newOrder
}

// getOrCreateInflight returns an existing inflight entry to wait on, or nil if this caller should proceed.
func (v *Capture) getOrCreateInflight(key string) *a11yInflightEntry {
	v.mu.Lock()
	defer v.mu.Unlock()

	if existing, ok := v.a11y.inflight[key]; ok {
		return existing
	}
	v.a11y.inflight[key] = &a11yInflightEntry{
		done: make(chan struct{}),
	}
	return nil
}

// completeInflight signals waiters and removes the inflight entry
func (v *Capture) completeInflight(key string, result json.RawMessage, err error) {
	v.mu.Lock()
	entry, exists := v.a11y.inflight[key]
	if exists {
		entry.result = result
		entry.err = err
		delete(v.a11y.inflight, key)
	}
	v.mu.Unlock()

	if exists {
		close(entry.done)
	}
}

// ExpireA11yCache forces all cache entries to expire (for testing)
func (v *Capture) ExpireA11yCache() {
	v.mu.Lock()
	defer v.mu.Unlock()

	for key, entry := range v.a11y.cache {
		entry.createdAt = time.Now().Add(-a11yCacheTTL - time.Second)
		v.a11y.cache[key] = entry
	}
}

// GetA11yCacheSize returns the number of entries in the a11y cache
func (v *Capture) GetA11yCacheSize() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.a11y.cache)
}

// SetLastKnownURL updates the last known page URL for navigation detection.
func (v *Capture) SetLastKnownURL(url string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.a11y.lastURL != "" && url != v.a11y.lastURL {
		v.a11y.cache = make(map[string]*a11yCacheEntry)
		v.a11y.cacheOrder = make([]string, 0)
	}
	v.a11y.lastURL = url
}
