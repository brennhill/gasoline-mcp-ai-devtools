package main

import (
	"encoding/json"
	"io"
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
		v.networkBodies = append(v.networkBodies, bodies[i])
		v.networkAddedAt = append(v.networkAddedAt, now)
	}

	// Enforce max count
	if len(v.networkBodies) > maxNetworkBodies {
		v.networkBodies = v.networkBodies[len(v.networkBodies)-maxNetworkBodies:]
		v.networkAddedAt = v.networkAddedAt[len(v.networkAddedAt)-maxNetworkBodies:]
	}

	// Enforce memory limit
	v.evictNBForMemory()
}

// evictNBForMemory removes oldest bodies if memory exceeds limit
func (v *Capture) evictNBForMemory() {
	for v.calcNBMemory() > nbBufferMemoryLimit && len(v.networkBodies) > 0 {
		v.networkBodies = v.networkBodies[1:]
		if len(v.networkAddedAt) > 0 {
			v.networkAddedAt = v.networkAddedAt[1:]
		}
	}
}

// calcNBMemory approximates memory usage of network bodies buffer
func (v *Capture) calcNBMemory() int64 {
	var total int64
	for _, b := range v.networkBodies {
		total += int64(len(b.RequestBody) + len(b.ResponseBody) + len(b.URL) + len(b.Method) + 64)
	}
	return total
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
	for _, b := range v.networkBodies {
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

	// Reverse for newest first
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	// Apply limit
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered
}

func (v *Capture) HandleNetworkBodies(w http.ResponseWriter, r *http.Request) {
	v.mu.RLock()
	memExceeded := v.isMemoryExceeded()
	v.mu.RUnlock()

	if memExceeded {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var payload struct {
		Bodies []NetworkBody `json:"bodies"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
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

	result := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": contentText},
		},
	}
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}
