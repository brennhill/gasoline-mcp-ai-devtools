// handlers.go — HTTP handlers for extension ↔ server communication
// Implements endpoints for pending queries, query results, and extension status.
// Part of the async queue-and-poll architecture (see internal/capture/queries.go).
package capture

import (
	"encoding/json"
	"net/http"
)

// HandleNetworkBodies returns network request bodies
func (c *Capture) HandleNetworkBodies(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	bodies := c.GetNetworkBodies()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"bodies": bodies,
		"count":  len(bodies),
	})
}

// HandleNetworkWaterfall returns network waterfall data
func (c *Capture) HandleNetworkWaterfall(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	bodies := c.GetNetworkBodies()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"waterfall": bodies,
		"count":     len(bodies),
	})
}

// HandlePendingQueries returns pending DOM queries for extension to execute.
// Extension polls this endpoint every 1-2 seconds to pick up commands.
func (c *Capture) HandlePendingQueries(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Check for client ID in multi-client mode
	clientID := r.Header.Get("X-Gasoline-Client")

	var queries interface{}
	if clientID != "" {
		queries = c.GetPendingQueriesForClient(clientID)
	} else {
		queries = c.GetPendingQueries()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"queries": queries,
		"count":   len(queries.([]PendingQueryResponse)),
	})
}

// HandlePilotStatus returns AI Pilot status
func (c *Capture) HandlePilotStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	c.mu.RLock()
	pilotEnabled := c.pilotEnabled
	c.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled": pilotEnabled,
	})
}

// HandleDOMResult processes DOM query results from extension.
// Extension posts results after executing DOM queries.
func (c *Capture) HandleDOMResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		ID       string          `json:"id"`
		Result   json.RawMessage `json:"result"`
		ClientID string          `json:"client_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	if body.ID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Missing query ID"})
		return
	}

	// Store result
	c.SetQueryResultWithClient(body.ID, body.Result, body.ClientID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
	})
}

// HandleA11yResult processes accessibility audit results from extension.
func (c *Capture) HandleA11yResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		ID       string          `json:"id"`
		Result   json.RawMessage `json:"result"`
		ClientID string          `json:"client_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	if body.ID != "" {
		c.SetQueryResultWithClient(body.ID, body.Result, body.ClientID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
	})
}

// HandleStateResult processes page state snapshots from extension.
func (c *Capture) HandleStateResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		ID       string          `json:"id"`
		Result   json.RawMessage `json:"result"`
		ClientID string          `json:"client_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	if body.ID != "" {
		c.SetQueryResultWithClient(body.ID, body.Result, body.ClientID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
	})
}

// HandleExecuteResult processes script execution results from extension.
func (c *Capture) HandleExecuteResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		ID       string          `json:"id"`
		Result   json.RawMessage `json:"result"`
		ClientID string          `json:"client_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	if body.ID != "" {
		c.SetQueryResultWithClient(body.ID, body.Result, body.ClientID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
	})
}

// HandleHighlightResult processes element highlight results from extension.
func (c *Capture) HandleHighlightResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		ID       string          `json:"id"`
		Result   json.RawMessage `json:"result"`
		ClientID string          `json:"client_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	if body.ID != "" {
		c.SetQueryResultWithClient(body.ID, body.Result, body.ClientID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
	})
}

// HandleEnhancedActions returns enhanced action events
func (c *Capture) HandleEnhancedActions(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	actions := c.GetAllEnhancedActions()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"actions": actions,
		"count":   len(actions),
	})
}

// HandlePerformanceSnapshots returns performance data snapshots
func (c *Capture) HandlePerformanceSnapshots(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"snapshots": []interface{}{},
		"count":     0,
	})
}
