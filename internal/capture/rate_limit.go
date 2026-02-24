// Purpose: Delegates capture rate-limiting and health endpoint behavior to the circuit-breaker subsystem.
// Why: Keeps ingest throttling logic centralized while retaining capture package API stability.
// Docs: docs/features/feature/rate-limiting/index.md

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
