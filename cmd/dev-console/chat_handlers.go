// chat_handlers.go — SSE streaming endpoint and sampling response handler for chat panel.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/push"
)

// handleChatStream serves an SSE connection for real-time chat messages.
// GET /chat/stream?conversation_id=...
func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "chat_stream: method not allowed. Use GET method."})
		return
	}

	convID := r.URL.Query().Get("conversation_id")
	if convID == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "chat_stream: conversation_id query parameter is required."})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "chat_stream: streaming not supported."})
		return
	}

	// Get or create session
	session := s.getOrCreateChatSession(convID)

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	// Send existing history as initial burst
	history := session.Messages()
	if len(history) > 0 {
		historyJSON, err := json.Marshal(history)
		if err == nil {
			fmt.Fprintf(w, "event: history\ndata: %s\n\n", historyJSON)
			flusher.Flush()
		}
	}

	// Subscribe for new messages
	ch, unsub := session.Subscribe()
	defer unsub()

	// Heartbeat ticker
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return // session closed
			}
			msgJSON, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", msgJSON)
			flusher.Flush()

		case <-heartbeat.C:
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()

		case <-ctx.Done():
			return
		}
	}
}

// handleChatResponse receives a sampling response forwarded by the bridge.
// POST /chat/response
func (s *Server) handleChatResponse(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "chat_response: method not allowed. Use POST method."})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)

	var body struct {
		RequestID int64  `json:"request_id"`
		Text      string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "chat_response: invalid JSON body."})
		return
	}

	// Look up conversation ID from tracked sampling requests
	convIDVal, ok := s.samplingRequests.LoadAndDelete(body.RequestID)
	if !ok {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "chat_response: no tracked conversation for this request ID."})
		return
	}
	convID, ok2 := convIDVal.(string)
	if !ok2 {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "chat_response: internal type error."})
		return
	}

	// Add assistant message to the session
	s.chatSessionMu.Lock()
	session := s.chatSession
	s.chatSessionMu.Unlock()

	if session != nil && session.ConversationID() == convID {
		session.AddMessage(push.ChatMessage{
			Role: push.ChatRoleAssistant,
			Text: body.Text,
		})
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}
