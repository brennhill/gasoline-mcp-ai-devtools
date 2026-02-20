// rate_limit.go — Capture delegation methods for rate limiting and circuit breaker.
// Delegates to CircuitBreaker sub-struct (aliased from internal/circuit).
// These methods preserve the existing Capture API so callers don't need to change.
package capture

import (
	"encoding/json"
	"net/http"

	"github.com/dev-console/dev-console/internal/util"
)

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
