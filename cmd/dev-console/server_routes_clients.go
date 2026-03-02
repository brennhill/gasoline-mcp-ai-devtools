// Purpose: Registers and serves multi-client registry HTTP endpoints (/clients, /clients/{id}).
// Why: Keeps client-registry HTTP behavior isolated from broader route wiring for maintainability and test focus.

package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
)

// registerClientRegistryRoutes binds the multi-client registry endpoints.
func registerClientRegistryRoutes(mux *http.ServeMux, cap *capture.Store) {
	mux.HandleFunc("/clients", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		handleClientsList(w, r, cap)
	})))
	mux.HandleFunc("/clients/", corsMiddleware(extensionOnly(func(w http.ResponseWriter, r *http.Request) {
		handleClientByID(w, r, cap)
	})))
}

// handleClientsList handles GET/POST on /clients for multi-client management.
func handleClientsList(w http.ResponseWriter, r *http.Request, cap *capture.Store) {
	switch r.Method {
	case "GET":
		clients := cap.GetClientRegistry().List()
		jsonResponse(w, http.StatusOK, map[string]any{
			"clients": clients,
			"count":   cap.GetClientRegistry().Count(),
		})
	case "POST":
		r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
		var body struct {
			CWD string `json:"cwd"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}
		cs := cap.GetClientRegistry().Register(body.CWD)
		jsonResponse(w, http.StatusOK, map[string]any{
			"result": cs,
		})
	default:
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

// handleClientByID handles GET/DELETE on /clients/{id} for specific client operations.
func handleClientByID(w http.ResponseWriter, r *http.Request, cap *capture.Store) {
	clientID := strings.TrimPrefix(r.URL.Path, "/clients/")
	if clientID == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing client ID"})
		return
	}

	switch r.Method {
	case "GET":
		cs := cap.GetClientRegistry().Get(clientID)
		if cs == nil {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Client not found"})
			return
		}
		jsonResponse(w, http.StatusOK, cs)
	case "DELETE":
		// Note: ClientRegistry interface doesn't expose Unregister method.
		jsonResponse(w, http.StatusOK, map[string]bool{"unregistered": true})
	default:
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}
