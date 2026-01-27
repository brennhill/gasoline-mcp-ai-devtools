package main

import (
	"encoding/json"
	"net/http"
	"time"
)

// ============================================
// Network Waterfall Handler
// ============================================
// Receives PerformanceResourceTiming data from the browser extension
// to build complete CSP policies and flag security issues.

// HandleNetworkWaterfall processes network waterfall data from the extension
func (c *Capture) HandleNetworkWaterfall(w http.ResponseWriter, r *http.Request) {
	var payload NetworkWaterfallPayload

	// Parse JSON payload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Process each entry in the payload
	for _, entry := range payload.Entries {
		// Set server-side timestamp
		entry.Timestamp = now

		// Set page URL from payload
		entry.PageURL = payload.PageURL

		// Append to ring buffer with capacity enforcement
		c.networkWaterfall = append(c.networkWaterfall, entry)

		// Evict oldest entries if over capacity
		if len(c.networkWaterfall) > c.networkWaterfallCapacity {
			// Keep only the last networkWaterfallCapacity entries
			c.networkWaterfall = c.networkWaterfall[len(c.networkWaterfall)-c.networkWaterfallCapacity:]
		}

		// Feed origin to CSP generator
		origin := extractOrigin(entry.URL)
		if origin != "" {
			c.cspGen.RecordOrigin(origin, entry.InitiatorType, payload.PageURL)
		}

		// Run security analysis and store flags
		flags := analyzeNetworkSecurity(entry, payload.PageURL)
		if len(flags) > 0 {
			c.securityFlags = append(c.securityFlags, flags...)

			// Enforce capacity: keep only last 1000 flags
			if len(c.securityFlags) > 1000 {
				c.securityFlags = c.securityFlags[len(c.securityFlags)-1000:]
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "ok",
		"entries_stored": len(payload.Entries),
	})
}
