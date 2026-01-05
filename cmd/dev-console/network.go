// network.go — Network request/response body storage and retrieval.
// Stores full HTTP payloads (request + response bodies) for API debugging,
// with ring buffer eviction and per-entry size limits.
// Design: Bodies indexed by request URL+method for fast lookup. Large
// payloads truncated to prevent memory bloat. Auth headers stripped on ingest.
package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// ============================================
// Network Bodies
// ============================================

// AddNetworkBodies adds network bodies to the buffer
func (v *Capture) AddNetworkBodies(bodies []NetworkBody) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Enforce memory limits before adding
	v.enforceMemory()

	v.networkTotalAdded += int64(len(bodies))
	now := time.Now()
	for i := range bodies {
		// Truncate request body
		if len(bodies[i].RequestBody) > maxRequestBodySize {
			bodies[i].RequestBody = bodies[i].RequestBody[:maxRequestBodySize] //nolint:gosec // G602: i is bounded by range
			bodies[i].RequestTruncated = true
		}
		// Truncate response body
		if len(bodies[i].ResponseBody) > maxResponseBodySize {
			bodies[i].ResponseBody = bodies[i].ResponseBody[:maxResponseBodySize] //nolint:gosec // G602: i is bounded by range
			bodies[i].ResponseTruncated = true
		}
		// Detect binary format in response body
		if bodies[i].BinaryFormat == "" && len(bodies[i].ResponseBody) > 0 {
			if format := DetectBinaryFormat([]byte(bodies[i].ResponseBody)); format != nil {
				bodies[i].BinaryFormat = format.Name
				bodies[i].FormatConfidence = format.Confidence
			}
		}
		v.networkBodies = append(v.networkBodies, bodies[i])
		v.networkAddedAt = append(v.networkAddedAt, now)
	}

	// Enforce max count (respecting minimal mode)
	capacity := v.effectiveNBCapacity()
	if len(v.networkBodies) > capacity {
		v.networkBodies = v.networkBodies[len(v.networkBodies)-capacity:]
		v.networkAddedAt = v.networkAddedAt[len(v.networkAddedAt)-capacity:]
	}

	// Enforce per-buffer memory limit
	v.evictNBForMemory()

	// Notify schema inference (non-blocking, separate lock)
	if v.schemaStore != nil || v.cspGen != nil {
		bodiesCopy := make([]NetworkBody, len(bodies))
		copy(bodiesCopy, bodies)
		if v.schemaStore != nil {
			go func() {
				for _, b := range bodiesCopy {
					v.schemaStore.Observe(b)
				}
			}()
		}
		// Feed CSP origin accumulator (non-blocking, separate lock)
		if v.cspGen != nil {
			go func() {
				for _, b := range bodiesCopy {
					// Use the request URL's origin as pageURL for non-HTML;
					// for HTML responses (the page itself), use the request URL
					v.cspGen.RecordOriginFromBody(b, b.URL)
				}
			}()
		}
	}
}

// evictNBForMemory removes oldest bodies if memory exceeds limit.
// Calculates how many entries to drop in a single pass to avoid O(n²) re-scanning.
func (v *Capture) evictNBForMemory() {
	excess := v.calcNBMemory() - nbBufferMemoryLimit
	if excess <= 0 {
		return
	}
	drop := 0
	for drop < len(v.networkBodies) && excess > 0 {
		excess -= int64(len(v.networkBodies[drop].RequestBody)+len(v.networkBodies[drop].ResponseBody)) + networkBodyOverhead
		drop++
	}
	v.networkBodies = v.networkBodies[drop:]
	if len(v.networkAddedAt) >= drop {
		v.networkAddedAt = v.networkAddedAt[drop:]
	}
}

// GetNetworkBodyCount returns the current number of buffered bodies
func (v *Capture) GetNetworkBodyCount() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.networkBodies)
}

// GetNetworkBodies returns filtered network bodies (newest first)
func (v *Capture) GetNetworkBodies(filter NetworkBodyFilter) []NetworkBody {
	v.mu.RLock()
	defer v.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultBodyLimit
	}

	var filtered []NetworkBody
	for i, b := range v.networkBodies {
		// TTL filtering: skip entries older than TTL
		if v.TTL > 0 && i < len(v.networkAddedAt) && isExpiredByTTL(v.networkAddedAt[i], v.TTL) {
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

func (v *Capture) HandleNetworkBodies(w http.ResponseWriter, r *http.Request) {
	body, ok := v.readIngestBody(w, r)
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
	if !v.recordAndRecheck(w, len(payload.Bodies)) {
		return
	}
	v.AddNetworkBodies(payload.Bodies)
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

	var contentText string
	if len(bodies) == 0 {
		contentText = "No network bodies captured"
	} else {
		bodiesJSON, _ := json.Marshal(bodies)
		contentText = string(bodiesJSON)
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(contentText)}
}
