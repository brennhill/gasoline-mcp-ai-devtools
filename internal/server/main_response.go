// Purpose: Implements shared HTTP JSON response helper(s) for server handlers.
// Why: Centralizes JSON write behavior and error logging for consistent responses.
// Docs: docs/features/feature/backend-log-streaming/index.md

package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// jsonResponse is a JSON response helper.
func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Error encoding JSON response: %v\n", err)
	}
}
