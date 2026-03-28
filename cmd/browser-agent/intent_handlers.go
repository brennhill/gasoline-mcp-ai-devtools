// intent_handlers.go — HTTP handlers for QA intent creation and terminal injection.
// Why: Bridges the extension's "Find Problems" button to the AI via PTY or intent fallback.
// Docs: docs/features/feature/auto-fix/index.md

package main

import (
	"encoding/json"
	"net/http"
)

type intentRequest struct {
	PageURL string `json:"page_url"`
	Action  string `json:"action"`
}

// registerIntentRoutes adds intent-related routes to the terminal mux.
func registerIntentRoutes(mux *http.ServeMux, server *Server) {
	// Inject text directly into the active PTY session.
	mux.HandleFunc("/terminal/inject", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handleTerminalInject(w, r, server)
	}))

	// Store an intent for the AI to pick up via MCP tool responses.
	mux.HandleFunc("/intent", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handleIntentCreate(w, r, server)
	}))
}

func handleTerminalInject(w http.ResponseWriter, r *http.Request, server *Server) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Text == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "missing text field"})
		return
	}

	if server.ptyRelays == nil {
		jsonResponse(w, http.StatusServiceUnavailable, map[string]any{
			"injected": false,
			"reason":   "no_terminal_server",
		})
		return
	}

	ok := server.ptyRelays.writeToFirst([]byte(body.Text + "\n"))
	if !ok {
		jsonResponse(w, http.StatusServiceUnavailable, map[string]any{
			"injected": false,
			"reason":   "no_active_session",
		})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{"injected": true})
}

func handleIntentCreate(w http.ResponseWriter, r *http.Request, server *Server) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	var req intentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Action == "" {
		req.Action = intentActionQAScan
	}

	if server.intentStore == nil {
		jsonResponse(w, http.StatusServiceUnavailable, map[string]string{"error": "intent store not initialized"})
		return
	}

	id := server.intentStore.Add(req.PageURL, req.Action)
	jsonResponse(w, http.StatusOK, map[string]any{
		"correlation_id": id,
		"stored":         true,
	})
}
