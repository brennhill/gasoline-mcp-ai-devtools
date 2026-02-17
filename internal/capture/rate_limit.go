// Purpose: Owns rate_limit.go runtime behavior and integration logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

// rate_limit.go — Capture delegation methods for rate limiting and circuit breaker.
// Delegates to CircuitBreaker sub-struct. These methods preserve the existing
// Capture API so callers (helpers.go, handlers, tests) don't need to change.
package capture

import (
	"encoding/json"
	"net/http"

	"github.com/dev-console/dev-console/internal/util"
)

// HealthResponse is returned by GET /health endpoint.
type HealthResponse struct {
	CircuitOpen bool   `json:"circuit_open"`
	OpenedAt    string `json:"opened_at,omitempty"`
	CurrentRate int    `json:"current_rate"`
	Reason      string `json:"reason,omitempty"`
}

// RateLimitResponse is the 429 response body.
type RateLimitResponse struct {
	Error        string `json:"error"`
	Message      string `json:"message"`
	RetryAfterMs int    `json:"retry_after_ms"`
	CircuitOpen  bool   `json:"circuit_open"`
	CurrentRate  int    `json:"current_rate"`
	Threshold    int    `json:"threshold"`
}

// RecordEvents delegates to CircuitBreaker.
func (c *Capture) RecordEvents(count int) {
	c.circuit.RecordEvents(count)
}

// CheckRateLimit delegates to CircuitBreaker.
func (c *Capture) CheckRateLimit() bool {
	return c.circuit.CheckRateLimit()
}

// GetHealthStatus delegates to CircuitBreaker.
func (c *Capture) GetHealthStatus() HealthResponse {
	return c.circuit.GetHealthStatus()
}

// WriteRateLimitResponse delegates to CircuitBreaker.
func (c *Capture) WriteRateLimitResponse(w http.ResponseWriter) {
	c.circuit.WriteRateLimitResponse(w)
}

// HandleHealth returns circuit breaker state as a JSON response (used by /health).
func (c *Capture) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		util.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	health := c.GetHealthStatus()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	//nolint:errcheck // HTTP response encoding errors are logged by client
	_ = json.NewEncoder(w).Encode(health)
}
