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

// ============================================
// Rate Limiting & Memory
// ============================================

// RecordEventReceived records an event for rate limiting
func (v *Capture) RecordEventReceived() {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	if now.Sub(v.rateResetTime) > time.Second {
		v.eventCount = 0
		v.rateResetTime = now
	}
	v.eventCount++
}

// SetMemoryUsage sets simulated memory usage for testing
func (v *Capture) SetMemoryUsage(bytes int64) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.simulatedMemory = bytes
}

func (v *Capture) HandlePendingQueries(w http.ResponseWriter, r *http.Request) {
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

// handleQueryResult handles a query result POST (shared between DOM and A11y)
func (v *Capture) handleQueryResult(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
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
		errResult := map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "Timeout waiting for DOM query result"},
			},
			"isError": true,
		}
		resultJSON, _ := json.Marshal(errResult)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
	}

	resp := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": string(result)},
		},
	}
	resultJSON, _ := json.Marshal(resp)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}

func (h *ToolHandler) toolGetPageInfo(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	id := h.capture.CreatePendingQuery(PendingQuery{
		Type:   "page_info",
		Params: json.RawMessage(`{}`),
	})

	result, err := h.capture.WaitForResult(id, h.capture.queryTimeout)
	if err != nil {
		errResult := map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "Timeout waiting for page info"},
			},
			"isError": true,
		}
		resultJSON, _ := json.Marshal(errResult)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
	}

	resp := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": string(result)},
		},
	}
	resultJSON, _ := json.Marshal(resp)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
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
			resp := map[string]interface{}{
				"content": []map[string]string{
					{"type": "text", "text": string(cached)},
				},
			}
			resultJSON, _ := json.Marshal(resp)
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
		}

		// Check if there's an inflight request for this key (concurrent dedup)
		if inflight := h.capture.getOrCreateInflight(cacheKey); inflight != nil {
			// Wait for the inflight request to complete
			<-inflight.done
			if inflight.err != nil {
				errResult := map[string]interface{}{
					"content": []map[string]string{
						{"type": "text", "text": "Timeout waiting for accessibility audit result"},
					},
					"isError": true,
				}
				resultJSON, _ := json.Marshal(errResult)
				return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
			}
			resp := map[string]interface{}{
				"content": []map[string]string{
					{"type": "text", "text": string(inflight.result)},
				},
			}
			resultJSON, _ := json.Marshal(resp)
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
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
		errResult := map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "Timeout waiting for accessibility audit result"},
			},
			"isError": true,
		}
		resultJSON, _ := json.Marshal(errResult)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
	}

	// Cache the successful result
	h.capture.setA11yCacheEntry(cacheKey, result)
	h.capture.completeInflight(cacheKey, result, nil)

	resp := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": string(result)},
		},
	}
	resultJSON, _ := json.Marshal(resp)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
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

	entry, exists := v.a11yCache[key]
	if !exists {
		return nil
	}
	if time.Since(entry.createdAt) > a11yCacheTTL {
		return nil
	}
	if v.lastKnownURL != "" && entry.url != "" && entry.url != v.lastKnownURL {
		return nil
	}
	return entry.result
}

// setA11yCacheEntry stores a result in the cache with eviction
func (v *Capture) setA11yCacheEntry(key string, result json.RawMessage) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if _, exists := v.a11yCache[key]; !exists && len(v.a11yCache) >= maxA11yCacheEntries {
		if len(v.a11yCacheOrder) > 0 {
			oldest := v.a11yCacheOrder[0]
			v.a11yCacheOrder = v.a11yCacheOrder[1:]
			delete(v.a11yCache, oldest)
		}
	}

	v.a11yCache[key] = &a11yCacheEntry{
		result:    result,
		createdAt: time.Now(),
		url:       v.lastKnownURL,
	}

	newOrder := make([]string, 0, len(v.a11yCacheOrder)+1)
	for _, k := range v.a11yCacheOrder {
		if k != key {
			newOrder = append(newOrder, k)
		}
	}
	newOrder = append(newOrder, key)
	v.a11yCacheOrder = newOrder
}

// removeA11yCacheEntry removes a specific cache entry
func (v *Capture) removeA11yCacheEntry(key string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	delete(v.a11yCache, key)
	newOrder := make([]string, 0, len(v.a11yCacheOrder))
	for _, k := range v.a11yCacheOrder {
		if k != key {
			newOrder = append(newOrder, k)
		}
	}
	v.a11yCacheOrder = newOrder
}

// getOrCreateInflight returns an existing inflight entry to wait on, or nil if this caller should proceed.
func (v *Capture) getOrCreateInflight(key string) *a11yInflightEntry {
	v.mu.Lock()
	defer v.mu.Unlock()

	if existing, ok := v.a11yInflight[key]; ok {
		return existing
	}
	v.a11yInflight[key] = &a11yInflightEntry{
		done: make(chan struct{}),
	}
	return nil
}

// completeInflight signals waiters and removes the inflight entry
func (v *Capture) completeInflight(key string, result json.RawMessage, err error) {
	v.mu.Lock()
	entry, exists := v.a11yInflight[key]
	if exists {
		entry.result = result
		entry.err = err
		delete(v.a11yInflight, key)
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

	for key, entry := range v.a11yCache {
		entry.createdAt = time.Now().Add(-a11yCacheTTL - time.Second)
		v.a11yCache[key] = entry
	}
}

// GetA11yCacheSize returns the number of entries in the a11y cache
func (v *Capture) GetA11yCacheSize() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.a11yCache)
}

// SetLastKnownURL updates the last known page URL for navigation detection.
func (v *Capture) SetLastKnownURL(url string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.lastKnownURL != "" && url != v.lastKnownURL {
		v.a11yCache = make(map[string]*a11yCacheEntry)
		v.a11yCacheOrder = make([]string, 0)
	}
	v.lastKnownURL = url
}
