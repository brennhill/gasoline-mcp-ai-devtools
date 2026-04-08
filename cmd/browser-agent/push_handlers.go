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

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

// ============================================
// Wire Types — checked by scripts/check-sync-wire-drift.js
// TS counterpart: src/types/wire-push.ts
// ============================================

// PushScreenshotRequest is the request body for POST /push/screenshot.
type PushScreenshotRequest struct {
	ScreenshotDataURL string `json:"screenshot_data_url"`
	Note              string `json:"note"`
	PageURL           string `json:"page_url"`
	TabID             int    `json:"tab_id"`
}

// PushMessageRequest is the request body for POST /push/message.
type PushMessageRequest struct {
	Message string `json:"message"`
	PageURL string `json:"page_url"`
	TabID   int    `json:"tab_id"`
}

// PushCapabilitiesResponse is the response from GET /push/capabilities.
type PushCapabilitiesResponse struct {
	PushEnabled           bool   `json:"push_enabled"`
	SupportsSampling      bool   `json:"supports_sampling"`
	SupportsNotifications bool   `json:"supports_notifications"`
	ClientName            string `json:"client_name"`
	InboxCount            int    `json:"inbox_count"`
}

// PushResponse is the response from POST /push/screenshot and POST /push/message.
type PushResponse struct {
	Status         string `json:"status"`
	EventID        string `json:"event_id"`
	DeliveryMethod string `json:"delivery_method"`
}

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

// validatePushRequest checks method and content-type for push POST endpoints.
// Returns true if the request is valid and the caller should proceed.
func validatePushRequest(w http.ResponseWriter, r *http.Request, errPrefix string) bool {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": errPrefix + ": method not allowed. Use POST method."})
		return false
	}
	contentType := r.Header.Get("Content-Type")
	if contentType != "" && !strings.HasPrefix(contentType, "application/json") {
		jsonResponse(w, http.StatusUnsupportedMediaType, map[string]string{"error": errPrefix + ": unsupported content type. Set Content-Type to application/json."})
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	return true
}

// deliverPushEvent routes a push event through the push router or inbox fallback,
// then writes the JSON response. Shared by handlePushScreenshot and handlePushMessage.
func (s *Server) deliverPushEvent(w http.ResponseWriter, ev push.PushEvent, errPrefix string) {
	status := "queued"
	deliveryMethod := string(push.DeliveredViaInbox)
	if s.pushRouter != nil {
		result, err := s.pushRouter.DeliverPush(ev)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": errPrefix + ": delivery failed. " + err.Error()})
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

// handlePushScreenshot receives a screenshot from the extension and routes it.
func (s *Server) handlePushScreenshot(w http.ResponseWriter, r *http.Request) {
	if !validatePushRequest(w, r, "push_screenshot") {
		return
	}

	var body PushScreenshotRequest
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

	s.deliverPushEvent(w, ev, "push_screenshot")
}

// handlePushMessage receives a chat message from the extension and routes it.
func (s *Server) handlePushMessage(w http.ResponseWriter, r *http.Request) {
	if !validatePushRequest(w, r, "push_message") {
		return
	}

	var body PushMessageRequest
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

	s.deliverPushEvent(w, ev, "push_message")
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

// handlePushDrain drains the push inbox and returns events as JSON.
// Called by the bridge process to relay push events to Claude via stdio.
func (s *Server) handlePushDrain(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.pushInbox == nil {
		jsonResponse(w, http.StatusOK, map[string]any{"events": []any{}, "count": 0})
		return
	}
	events := s.pushInbox.DrainAll()
	if len(events) == 0 {
		jsonResponse(w, http.StatusOK, map[string]any{"events": []any{}, "count": 0})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"events": events, "count": len(events)})
}

// pushDrawModeCompletion builds an annotation PushEvent and routes it.
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

	_, _ = s.pushRouter.DeliverPush(ev)
}
