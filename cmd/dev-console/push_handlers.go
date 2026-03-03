// push_handlers.go — HTTP handlers for push screenshot, chat, and capabilities.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/push"
)

// pushEventID generates a unique event ID with the given prefix.
func pushEventID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback to time-based ID on crypto failure (extremely rare)
		ts := time.Now().UnixNano()
		return prefix + "-" + fmt.Sprintf("%x", ts)
	}
	return prefix + "-" + hex.EncodeToString(b[:])
}

// handlePushScreenshot receives a screenshot from the extension and routes it.
func (s *Server) handlePushScreenshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "push_screenshot: method not allowed. Use POST method."})
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "" && !strings.HasPrefix(contentType, "application/json") {
		jsonResponse(w, http.StatusUnsupportedMediaType, map[string]string{"error": "push_screenshot: unsupported content type. Set Content-Type to application/json."})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	var body struct {
		ScreenshotDataURL string `json:"screenshot_data_url"`
		Note              string `json:"note"`
		PageURL           string `json:"page_url"`
		TabID             int    `json:"tab_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "push_screenshot: invalid JSON body. Send a valid JSON object."})
		return
	}

	// Strip data URL prefix to get raw base64
	b64 := body.ScreenshotDataURL
	if idx := strings.Index(b64, ","); idx >= 0 {
		b64 = b64[idx+1:]
	}

	ev := push.PushEvent{
		ID:            pushEventID("push-ss"),
		Type:          "screenshot",
		Timestamp:     time.Now(),
		PageURL:       body.PageURL,
		TabID:         body.TabID,
		ScreenshotB64: b64,
		Note:          body.Note,
	}

	status := "queued"
	deliveryMethod := string(push.DeliveredViaInbox)
	if s.pushRouter != nil {
		result, err := s.pushRouter.DeliverPush(ev)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "push_screenshot: delivery failed. " + err.Error()})
			return
		}
		deliveryMethod = string(result.Method)
		if result.Method == push.DeliveredViaSampling {
			status = "delivered"
		}
	} else if s.pushInbox != nil {
		s.pushInbox.Enqueue(ev)
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"status":          status,
		"event_id":        ev.ID,
		"delivery_method": deliveryMethod,
	})
}

// handlePushMessage receives a chat message from the extension and routes it.
// If conversation_id is present, the message is also added to the active ChatSession
// and the sampling request ID is tracked for response correlation.
func (s *Server) handlePushMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "push_message: method not allowed. Use POST method."})
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "" && !strings.HasPrefix(contentType, "application/json") {
		jsonResponse(w, http.StatusUnsupportedMediaType, map[string]string{"error": "push_message: unsupported content type. Set Content-Type to application/json."})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	var body struct {
		Message        string `json:"message"`
		PageURL        string `json:"page_url"`
		TabID          int    `json:"tab_id"`
		ConversationID string `json:"conversation_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "push_message: invalid JSON body. Send a valid JSON object with a 'message' field."})
		return
	}

	if strings.TrimSpace(body.Message) == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "push_message: message field is empty. Provide a non-empty message."})
		return
	}

	ev := push.PushEvent{
		ID:        pushEventID("push-chat"),
		Type:      "chat",
		Timestamp: time.Now(),
		PageURL:   body.PageURL,
		TabID:     body.TabID,
		Message:   body.Message,
	}

	// If conversation_id is present, manage the chat session
	convID := body.ConversationID
	if convID != "" {
		session := s.getOrCreateChatSession(convID)
		session.AddMessage(push.ChatMessage{
			Role: push.ChatRoleUser,
			Text: body.Message,
		})
	}

	status := "queued"
	deliveryMethod := string(push.DeliveredViaInbox)
	if s.pushRouter != nil {
		if convID != "" {
			// Conversation mode: build request manually so we can track the ID
			req := push.BuildSamplingRequest(ev)
			result, err := s.pushRouter.DeliverPushWithRequest(ev, req)
			if err != nil {
				jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "push_message: delivery failed. " + err.Error()})
				return
			}
			deliveryMethod = string(result.Method)
			if result.Method == push.DeliveredViaSampling {
				status = "delivered"
				// Track request ID for response correlation with TTL cleanup
				s.samplingRequests.Store(req.ID, convID)
				time.AfterFunc(5*time.Minute, func() {
					s.samplingRequests.Delete(req.ID)
				})
			}
			// If sampling failed, don't track — the response will never come
		} else {
			// Normal mode: use standard delivery with fallback chain
			result, err := s.pushRouter.DeliverPush(ev)
			if err != nil {
				jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "push_message: delivery failed. " + err.Error()})
				return
			}
			deliveryMethod = string(result.Method)
			if result.Method == push.DeliveredViaSampling {
				status = "delivered"
			}
		}
	} else if s.pushInbox != nil {
		s.pushInbox.Enqueue(ev)
	}

	resp := map[string]any{
		"status":          status,
		"event_id":        ev.ID,
		"delivery_method": deliveryMethod,
	}
	if convID != "" {
		resp["conversation_id"] = convID
	}
	jsonResponse(w, http.StatusOK, resp)
}

// handlePushCapabilities returns per-session push capability flags for the extension.
func (s *Server) handlePushCapabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "push_capabilities: method not allowed. Use GET method."})
		return
	}

	caps := getPushClientCapabilities()
	inboxCount := 0
	if s.pushInbox != nil {
		inboxCount = s.pushInbox.Len()
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"push_enabled":           caps.SupportsSampling || caps.SupportsNotifications,
		"supports_sampling":      caps.SupportsSampling,
		"supports_notifications": caps.SupportsNotifications,
		"client_name":            caps.ClientName,
		"inbox_count":            inboxCount,
	})
}

// pushDrawModeCompletion builds an annotation PushEvent and routes it.
// If a chat session is active, also injects annotation data as a chat message.
func (s *Server) pushDrawModeCompletion(body *drawModeRequest, screenshotPath string, annotations []Annotation) {
	if s.pushRouter == nil {
		return
	}

	annotJSON, err := json.Marshal(annotations)
	if err != nil {
		return
	}

	ev := push.PushEvent{
		ID:           pushEventID("push-ann"),
		Type:         "annotations",
		Timestamp:    time.Now(),
		PageURL:      body.PageURL,
		TabID:        body.TabID,
		Annotations:  annotJSON,
		AnnotSession: body.AnnotSessionName,
	}

	// If chat session is active, inject annotations into the conversation
	s.chatSessionMu.Lock()
	session := s.chatSession
	s.chatSessionMu.Unlock()
	if session != nil {
		session.AddMessage(push.ChatMessage{
			Role:        push.ChatRoleAnnotation,
			Text:        fmt.Sprintf("%d annotations from draw mode", len(annotations)),
			Annotations: annotJSON,
		})
	}

	_, _ = s.pushRouter.DeliverPush(ev)
}

// getOrCreateChatSession returns the active chat session, creating one if needed.
func (s *Server) getOrCreateChatSession(conversationID string) *push.ChatSession {
	s.chatSessionMu.Lock()
	defer s.chatSessionMu.Unlock()

	if s.chatSession != nil && s.chatSession.ConversationID() == conversationID {
		return s.chatSession
	}

	// Close previous session if different conversation
	if s.chatSession != nil {
		s.chatSession.Close()
	}
	s.chatSession = push.NewChatSession(conversationID)
	return s.chatSession
}
