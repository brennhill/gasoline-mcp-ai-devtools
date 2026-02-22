// openapi.go — Serves the embedded OpenAPI 3.1.0 specification at GET /openapi.json.
// Docs: docs/features/feature/observe/index.md
package main

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.json
var openapiJSON []byte

// handleOpenAPI serves the OpenAPI specification.
func handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(openapiJSON); err != nil {
		stderrf("[gasoline] failed to write /openapi.json response: %v\n", err)
	}
}
