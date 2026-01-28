// network.go — Network request/response body storage and retrieval.
// Stores full HTTP payloads (request + response bodies) for API debugging,
// with ring buffer eviction and per-entry size limits.
// Design: Bodies indexed by request URL+method for fast lookup. Large
// payloads truncated to prevent memory bloat. Auth headers stripped on ingest.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ============================================
// Network Bodies
// ============================================

// AddNetworkBodies adds network bodies to the buffer
func (c *Capture) AddNetworkBodies(bodies []NetworkBody) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Enforce memory limits before adding
	c.enforceMemory()

	c.networkTotalAdded += int64(len(bodies))
	now := time.Now()
	for i := range bodies {
		// Truncate request body
		if len(bodies[i].RequestBody) > maxRequestBodySize {
			bodies[i].RequestBody = bodies[i].RequestBody[:maxRequestBodySize] // #nosec G602 -- i is bounded by range
			bodies[i].RequestTruncated = true
		}
		// Truncate response body
		if len(bodies[i].ResponseBody) > maxResponseBodySize {
			bodies[i].ResponseBody = bodies[i].ResponseBody[:maxResponseBodySize] // #nosec G602 -- i is bounded by range
			bodies[i].ResponseTruncated = true
		}
		// Detect binary format in response body
		if bodies[i].BinaryFormat == "" && len(bodies[i].ResponseBody) > 0 {
			if format := DetectBinaryFormat([]byte(bodies[i].ResponseBody)); format != nil {
				bodies[i].BinaryFormat = format.Name
				bodies[i].FormatConfidence = format.Confidence
			}
		}
		c.networkBodies = append(c.networkBodies, bodies[i])
		c.networkAddedAt = append(c.networkAddedAt, now)
		c.nbMemoryTotal += nbEntryMemory(&bodies[i])
	}

	// Enforce max count (respecting minimal mode)
	capacity := c.effectiveNBCapacity()
	if len(c.networkBodies) > capacity {
		keep := len(c.networkBodies) - capacity
		// Subtract memory for evicted entries
		for j := 0; j < keep; j++ {
			c.nbMemoryTotal -= nbEntryMemory(&c.networkBodies[j])
		}
		newBodies := make([]NetworkBody, capacity)
		copy(newBodies, c.networkBodies[keep:])
		c.networkBodies = newBodies
		newAddedAt := make([]time.Time, capacity)
		copy(newAddedAt, c.networkAddedAt[keep:])
		c.networkAddedAt = newAddedAt
	}

	// Enforce per-buffer memory limit
	c.evictNBForMemory()

	// Notify schema inference and CSP (non-blocking, separate locks, bounded)
	if c.schemaStore != nil || c.cspGen != nil {
		bodiesCopy := make([]NetworkBody, len(bodies))
		copy(bodiesCopy, bodies)
		select {
		case c.observeSem <- struct{}{}:
			go func() {
				defer func() { <-c.observeSem }()
				for _, b := range bodiesCopy {
					if c.schemaStore != nil {
						c.schemaStore.Observe(b)
					}
					if c.cspGen != nil {
						c.cspGen.RecordOriginFromBody(b, b.URL)
					}
				}
			}()
		default:
			// Too many observers in flight; drop to avoid goroutine accumulation
		}
	}
}

// evictNBForMemory removes oldest bodies if memory exceeds limit.
// Calculates how many entries to drop in a single pass to avoid O(n²) re-scanning.
func (c *Capture) evictNBForMemory() {
	excess := c.nbMemoryTotal - nbBufferMemoryLimit
	if excess <= 0 {
		return
	}
	drop := 0
	for drop < len(c.networkBodies) && excess > 0 {
		entryMem := nbEntryMemory(&c.networkBodies[drop])
		excess -= entryMem
		c.nbMemoryTotal -= entryMem
		drop++
	}
	surviving := make([]NetworkBody, len(c.networkBodies)-drop)
	copy(surviving, c.networkBodies[drop:])
	c.networkBodies = surviving
	if len(c.networkAddedAt) >= drop {
		survivingAt := make([]time.Time, len(c.networkAddedAt)-drop)
		copy(survivingAt, c.networkAddedAt[drop:])
		c.networkAddedAt = survivingAt
	}
}

// GetNetworkBodyCount returns the current number of buffered bodies
func (c *Capture) GetNetworkBodyCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.networkBodies)
}

// GetNetworkBodies returns filtered network bodies (newest first)
func (c *Capture) GetNetworkBodies(filter NetworkBodyFilter) []NetworkBody {
	c.mu.RLock()
	defer c.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultBodyLimit
	}

	var filtered []NetworkBody
	for i, b := range c.networkBodies {
		// TTL filtering: skip entries older than TTL
		if c.TTL > 0 && i < len(c.networkAddedAt) && isExpiredByTTL(c.networkAddedAt[i], c.TTL) {
			continue
		}
		if filter.URLFilter != "" && !strings.Contains(b.URL, filter.URLFilter) {
			continue
		}
		if filter.Method != "" && b.Method != filter.Method {
			continue
		}
		if filter.StatusMin > 0 && b.Status < filter.StatusMin {
			continue
		}
		if filter.StatusMax > 0 && b.Status > filter.StatusMax {
			continue
		}
		filtered = append(filtered, b)
	}

	reverseSlice(filtered)
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

func (c *Capture) HandleNetworkBodies(w http.ResponseWriter, r *http.Request) {
	body, ok := c.readIngestBody(w, r)
	if !ok {
		return
	}
	var payload struct {
		Bodies []NetworkBody `json:"bodies"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !c.recordAndRecheck(w, len(payload.Bodies)) {
		return
	}
	c.AddNetworkBodies(payload.Bodies)
	w.WriteHeader(http.StatusOK)
}

// HandlePendingQueries handles GET /pending-queries

func (h *ToolHandler) toolGetNetworkBodies(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		URL       string `json:"url"`
		Method    string `json:"method"`
		StatusMin int    `json:"status_min"`
		StatusMax int    `json:"status_max"`
		Limit     int    `json:"limit"`
	}
	_ = json.Unmarshal(args, &arguments) // Optional args - zero values are acceptable defaults

	bodies := h.capture.GetNetworkBodies(NetworkBodyFilter{
		URLFilter: arguments.URL,
		Method:    arguments.Method,
		StatusMin: arguments.StatusMin,
		StatusMax: arguments.StatusMax,
		Limit:     arguments.Limit,
	})

	// Apply noise filtering
	if h.noise != nil {
		var filtered []NetworkBody
		for _, b := range bodies {
			if !h.noise.IsNetworkNoise(b) {
				filtered = append(filtered, b)
			}
		}
		bodies = filtered
	}

	if len(bodies) == 0 {
		data := map[string]interface{}{
			"networkRequestResponsePairs": []interface{}{},
			"count":                       0,
			"maxRequestBodyBytes":         8192,
			"maxResponseBodyBytes":        16384,
		}
		if h.captureOverrides != nil {
			overrides := h.captureOverrides.GetAll()
			if overrides["network_bodies"] == "false" {
				data["hint"] = "Network body capture is OFF. To enable, call: configure({action: \"capture\", settings: {network_bodies: \"true\"}})"
			} else {
				data["hint"] = "No network bodies captured. Ensure: (1) a tab is being tracked via the extension's 'Track This Tab' button, (2) the page has made network requests since tracking started."
			}
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("", data)}
	}

	// Build JSON entries
	jsonPairs := make([]map[string]interface{}, len(bodies))
	for i, b := range bodies {
		jsonPairs[i] = map[string]interface{}{
			"url":        b.URL,
			"method":     b.Method,
			"status":     b.Status,
			"durationMs": b.Duration,
		}

		// Add timestamp if present (renamed capturedAt)
		if b.Timestamp != "" {
			jsonPairs[i]["capturedAt"] = b.Timestamp
		}

		// Add content type if present
		if b.ContentType != "" {
			jsonPairs[i]["contentType"] = b.ContentType
		}

		// Include request body if present
		if b.RequestBody != "" {
			jsonPairs[i]["requestBody"] = b.RequestBody
			jsonPairs[i]["requestBodySizeBytes"] = len(b.RequestBody)
			if b.RequestTruncated {
				jsonPairs[i]["requestBodyTruncated"] = true
			}
		}

		// Include response body if present
		if b.ResponseBody != "" {
			jsonPairs[i]["responseBody"] = b.ResponseBody
			jsonPairs[i]["responseBodySizeBytes"] = len(b.ResponseBody)
			if b.ResponseTruncated {
				jsonPairs[i]["responseBodyTruncated"] = true
			}
		}

		// Add response headers if present
		if len(b.ResponseHeaders) > 0 {
			jsonPairs[i]["responseHeaders"] = b.ResponseHeaders
		}

		// Add binary format detection if present with confidence interpretation
		if b.BinaryFormat != "" {
			jsonPairs[i]["binaryFormat"] = b.BinaryFormat
			jsonPairs[i]["binaryFormatConfidence"] = b.FormatConfidence

			// Add interpretation based on confidence level
			var interpretation string
			if b.FormatConfidence >= 0.8 {
				interpretation = "high_confidence"
			} else if b.FormatConfidence >= 0.5 {
				interpretation = "medium_confidence"
			} else {
				interpretation = "low_confidence"
			}
			jsonPairs[i]["binaryFormatInterpretation"] = interpretation
		}
	}

	data := map[string]interface{}{
		"networkRequestResponsePairs": jsonPairs,
		"count":                       len(bodies),
		"maxRequestBodyBytes":         8192,
		"maxResponseBodyBytes":        16384,
	}

	summary := fmt.Sprintf("%d network request-response pair(s)", len(bodies))
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, data)}
}

// toolGetNetworkWaterfall retrieves PerformanceResourceTiming waterfall data
func (h *ToolHandler) toolGetNetworkWaterfall(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Parse parameters
	var params struct {
		URL   string `json:"url"`   // Substring filter
		Limit int    `json:"limit"` // Max entries to return
	}
	json.Unmarshal(args, &params)

	h.capture.mu.RLock()
	entries := make([]NetworkWaterfallEntry, len(h.capture.networkWaterfall))
	copy(entries, h.capture.networkWaterfall)
	h.capture.mu.RUnlock()

	// Apply URL filter
	if params.URL != "" {
		filtered := []NetworkWaterfallEntry{}
		for _, entry := range entries {
			if strings.Contains(entry.URL, params.URL) {
				filtered = append(filtered, entry)
			}
		}
		entries = filtered
	}

	// Apply limit (last N entries)
	if params.Limit > 0 && len(entries) > params.Limit {
		entries = entries[len(entries)-params.Limit:]
	}

	// Empty buffer case
	if len(entries) == 0 {
		data := map[string]interface{}{
			"entries":     []interface{}{},
			"count":       0,
			"limitations": []string{
				"No HTTP status codes (use network_bodies for 404s/500s/401s)",
				"No request methods (GET/POST/etc.)",
				"No request/response headers or bodies",
			},
			"hint": "Network waterfall data comes from the PerformanceResourceTiming API. Ensure the extension is active and a page is loaded.",
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("", data)}
	}

	// Calculate time range
	oldestTime := entries[0].Timestamp
	newestTime := entries[len(entries)-1].Timestamp
	timeRange := newestTime.Sub(oldestTime)

	// Build JSON entries with unit suffixes and computed fields
	jsonEntries := make([]map[string]interface{}, len(entries))
	for i, entry := range entries {
		// Calculate compression ratio
		compressionRatio := 0.0
		if entry.DecodedBodySize > 0 {
			compressionRatio = float64(entry.EncodedBodySize) / float64(entry.DecodedBodySize)
		}

		jsonEntries[i] = map[string]interface{}{
			"url":                   entry.URL,
			"initiatorType":         entry.InitiatorType,
			"durationMs":            entry.Duration,
			"startTimeMs":           entry.StartTime,
			"fetchStartMs":          entry.FetchStart,
			"responseEndMs":         entry.ResponseEnd,
			"transferSizeBytes":     entry.TransferSize,
			"decodedBodySizeBytes":  entry.DecodedBodySize,
			"encodedBodySizeBytes":  entry.EncodedBodySize,
			"compressionRatio":      compressionRatio,
			"cached":                entry.TransferSize == 0 && entry.DecodedBodySize > 0,
			"pageURL":               entry.PageURL,
			"capturedAt":            entry.Timestamp.Format(time.RFC3339),
		}
	}

	data := map[string]interface{}{
		"entries":          jsonEntries,
		"count":            len(entries),
		"timespan":         formatDuration(timeRange),
		"oldestTimestamp":  oldestTime.Format(time.RFC3339),
		"newestTimestamp":  newestTime.Format(time.RFC3339),
		"limitations": []string{
			"No HTTP status codes (use network_bodies for 404s/500s/401s)",
			"No request methods (GET/POST/etc.)",
			"No request/response headers or bodies",
		},
	}

	summary := fmt.Sprintf("%d network waterfall entries (timespan: %s)", len(entries), formatDuration(timeRange))
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, data)}
}

// entryStr extracts a string value from a LogEntry map, returning "" if the
// key is missing or the value is not a string.
func entryStr(entry LogEntry, key string) string {
	v, ok := entry[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// entryDisplay extracts a value from a LogEntry map and returns its string
// representation. Unlike entryStr, it handles numeric types (e.g. tabId
// arrives as float64 from JSON).
func entryDisplay(entry LogEntry, key string) string {
	v, ok := entry[key]
	if !ok || v == nil {
		return ""
	}
	switch tv := v.(type) {
	case string:
		return tv
	case float64:
		if tv == float64(int64(tv)) {
			return fmt.Sprintf("%d", int64(tv))
		}
		return fmt.Sprintf("%g", tv)
	default:
		return fmt.Sprintf("%v", v)
	}
}
