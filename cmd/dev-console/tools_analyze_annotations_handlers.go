// Purpose: Handles analyze annotation modes for session reads and annotation detail.
// Why: Keeps annotation retrieval and detail formatting separate from draw-session file I/O.
// Docs: docs/features/feature/annotated-screenshots/index.md

package main

import (
	"encoding/json"
	"strings"
	"time"
)

// annotationWaitCommandTTL is how long pending annotation commands remain active.
const annotationWaitCommandTTL = 10 * time.Minute

// annotationBlockingWaitDefault is the initial synchronous wait budget for annotations(wait=true).
// If annotations arrive in this window, return them directly without requiring polling.
const annotationBlockingWaitDefault = 15 * time.Second

// annotationBlockingWaitMax caps caller-provided timeout_ms for wait=true annotation calls.
const annotationBlockingWaitMax = 10 * time.Minute

// toolGetAnnotations returns latest annotation session or a named multi-page session.
func (h *ToolHandler) toolGetAnnotations(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Wait         bool   `json:"wait"`
		AnnotSession string `json:"annot_session"`
		Operation    string `json:"operation"`
		Correlation  string `json:"correlation_id"`
		TimeoutMs    int    `json:"timeout_ms"`
	}
	if len(args) > 0 {
		lenientUnmarshal(args, &params)
	}

	operation := strings.ToLower(strings.TrimSpace(params.Operation))
	if operation != "" {
		if operation != "flush" {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  mcpStructuredError(ErrInvalidParam, "Invalid annotations operation: "+params.Operation, "Use operation='flush' for annotation waiter recovery.", withParam("operation"), withHint("flush")),
			}
		}
		return h.toolFlushAnnotations(req, params.Correlation)
	}

	waitTimeout := annotationBlockingWaitDuration(params.TimeoutMs)
	if params.AnnotSession != "" {
		return h.getNamedAnnotations(req, params.AnnotSession, params.Wait, waitTimeout)
	}
	return h.getAnonymousAnnotations(req, params.Wait, waitTimeout)
}

func annotationBlockingWaitDuration(timeoutMs int) time.Duration {
	if timeoutMs <= 0 {
		return annotationBlockingWaitDefault
	}
	waitDuration := time.Duration(timeoutMs) * time.Millisecond
	if waitDuration > annotationBlockingWaitMax {
		return annotationBlockingWaitMax
	}
	return waitDuration
}

func (h *ToolHandler) getAnonymousAnnotations(req JSONRPCRequest, wait bool, waitTimeout time.Duration) JSONRPCResponse {
	if wait {
		if session := h.annotationStore.GetLatestSessionSinceDraw(); session != nil {
			return h.formatAnnotationSession(req, session)
		}

		if session, _ := h.annotationStore.WaitForSession(waitTimeout); session != nil {
			return h.formatAnnotationSession(req, session)
		}

		corrID := newCorrelationID("ann")
		h.capture.RegisterCommand(corrID, "", annotationWaitCommandTTL)
		h.annotationStore.RegisterWaiter(corrID, "")

		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Waiting for annotations", map[string]any{
			"status":         "waiting_for_user",
			"correlation_id": corrID,
			"annotations":    []any{},
			"count":          0,
			"message":        "Draw mode is active. The user is drawing annotations. Poll with observe({what: 'command_result', correlation_id: '" + corrID + "'}) to check for results.",
		})}
	}

	session := h.annotationStore.GetLatestSession()
	if session == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("No annotations", map[string]any{
			"annotations": []any{},
			"count":       0,
			"message":     "No annotation session found. Use interact({action: 'draw_mode_start'}) to activate draw mode, then the user draws annotations and presses ESC to finish.",
		})}
	}
	return h.formatAnnotationSession(req, session)
}

func (h *ToolHandler) formatAnnotationSession(req JSONRPCRequest, session *AnnotationSession) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Annotations retrieved", buildAnnotationSessionResult(session))}
}

// #lizard forgives
func (h *ToolHandler) getNamedAnnotations(req JSONRPCRequest, sessionName string, wait bool, waitTimeout time.Duration) JSONRPCResponse {
	if wait {
		if ns := h.annotationStore.GetNamedSessionSinceDraw(sessionName); ns != nil {
			return h.formatNamedAnnotationSession(req, ns)
		}

		if ns, _ := h.annotationStore.WaitForNamedSession(sessionName, waitTimeout); ns != nil {
			return h.formatNamedAnnotationSession(req, ns)
		}

		corrID := newCorrelationID("ann")
		h.capture.RegisterCommand(corrID, "", annotationWaitCommandTTL)
		h.annotationStore.RegisterWaiter(corrID, sessionName)

		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Waiting for annotations", map[string]any{
			"status":             "waiting_for_user",
			"correlation_id":     corrID,
			"annot_session_name": sessionName,
			"pages":              []any{},
			"page_count":         0,
			"total_count":        0,
			"message":            "Draw mode is active. The user is drawing annotations. Poll with observe({what: 'command_result', correlation_id: '" + corrID + "'}) to check for results.",
		})}
	}

	ns := h.annotationStore.GetNamedSession(sessionName)
	if ns == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("No annotations", map[string]any{
			"annot_session_name": sessionName,
			"pages":              []any{},
			"page_count":         0,
			"total_count":        0,
			"message":            "Named session '" + sessionName + "' not found. Use interact({action: 'draw_mode_start', annot_session: '" + sessionName + "'}) to start.",
		})}
	}

	return h.formatNamedAnnotationSession(req, ns)
}

func (h *ToolHandler) formatNamedAnnotationSession(req JSONRPCRequest, ns *NamedAnnotationSession) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Annotations retrieved", buildNamedAnnotationSessionResult(ns))}
}

func buildAnnotationSessionResult(session *AnnotationSession) map[string]any {
	result := map[string]any{
		"annotations": session.Annotations,
		"count":       len(session.Annotations),
		"page_url":    session.PageURL,
	}
	if session.ScreenshotPath != "" {
		result["screenshot"] = session.ScreenshotPath
	}
	return result
}

func buildNamedAnnotationSessionResult(ns *NamedAnnotationSession) map[string]any {
	totalCount := 0
	pages := make([]map[string]any, 0, len(ns.Pages))
	for _, page := range ns.Pages {
		totalCount += len(page.Annotations)
		p := map[string]any{
			"page_url":    page.PageURL,
			"annotations": page.Annotations,
			"count":       len(page.Annotations),
			"tab_id":      page.TabID,
		}
		if page.ScreenshotPath != "" {
			p["screenshot"] = page.ScreenshotPath
		}
		pages = append(pages, p)
	}

	return map[string]any{
		"annot_session_name": ns.Name,
		"pages":              pages,
		"page_count":         len(ns.Pages),
		"total_count":        totalCount,
	}
}

// toolFlushAnnotations forces completion of a pending annotation waiter.
// This is a recovery path for stuck waiters that would otherwise remain pending.
func (h *ToolHandler) toolFlushAnnotations(req JSONRPCRequest, correlationID string) JSONRPCResponse {
	correlationID = strings.TrimSpace(correlationID)
	if correlationID == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrMissingParam,
			"Required parameter 'correlation_id' is missing for operation='flush'",
			"Pass the correlation_id returned by analyze({what:'annotations',wait:true}).",
			withParam("correlation_id"),
		)}
	}
	if !strings.HasPrefix(correlationID, "ann_") {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam,
			"Invalid annotation correlation_id: "+correlationID,
			"Use an annotation correlation_id (prefix ann_) from analyze({what:'annotations',wait:true}).",
			withParam("correlation_id"),
		)}
	}

	// Remove waiter first so later session writes cannot re-complete the same flush target.
	sessionName, _ := h.annotationStore.TakeWaiter(correlationID)

	// Idempotent behavior: if the command is already terminal, return current state.
	if cmd, found := h.capture.GetCommandResult(correlationID); found && cmd != nil && cmd.Status != "pending" {
		return h.formatCommandResult(req, *cmd, correlationID)
	}

	payload := h.buildFlushedAnnotationResult(sessionName)
	h.capture.ApplyCommandResult(correlationID, "complete", payload, "")

	// Normal path: return canonical command_result envelope.
	if cmd, found := h.capture.GetCommandResult(correlationID); found && cmd != nil {
		return h.formatCommandResult(req, *cmd, correlationID)
	}

	// Recovery fallback: command tracker no longer has this correlation_id.
	// Return flush payload directly so callers still receive deterministic output.
	var data map[string]any
	_ = json.Unmarshal(payload, &data)
	data["status"] = "complete"
	data["final"] = true
	data["correlation_id"] = correlationID
	data["lifecycle_status"] = "complete"
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Annotation flush completed", data)}
}

func (h *ToolHandler) buildFlushedAnnotationResult(sessionName string) json.RawMessage {
	if sessionName != "" {
		if ns := h.annotationStore.GetNamedSession(sessionName); ns != nil {
			data := buildNamedAnnotationSessionResult(ns)
			data["status"] = "complete"
			data["terminal_reason"] = "flushed"
			encoded, _ := json.Marshal(data)
			return encoded
		}

		encoded, _ := json.Marshal(map[string]any{
			"status":             "complete",
			"annot_session_name": sessionName,
			"pages":              []any{},
			"page_count":         0,
			"total_count":        0,
			"terminal_reason":    "abandoned",
			"message":            "Annotation waiter flushed with no named-session annotations available.",
		})
		return encoded
	}

	if session := h.annotationStore.GetLatestSession(); session != nil {
		data := buildAnnotationSessionResult(session)
		data["status"] = "complete"
		data["terminal_reason"] = "flushed"
		encoded, _ := json.Marshal(data)
		return encoded
	}

	encoded, _ := json.Marshal(map[string]any{
		"status":          "complete",
		"annotations":     []any{},
		"count":           0,
		"terminal_reason": "abandoned",
		"message":         "Annotation waiter flushed with no captured annotations available.",
	})
	return encoded
}

// toolGetAnnotationDetail returns full DOM/style detail for a specific annotation.
func (h *ToolHandler) toolGetAnnotationDetail(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		CorrelationID string `json:"correlation_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.CorrelationID == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'correlation_id' is missing", "Add the 'correlation_id' from the annotation you want detail for", withParam("correlation_id"))}
	}

	detail, found := h.annotationStore.GetDetail(params.CorrelationID)
	if !found {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "Annotation detail not found or expired for correlation_id: "+params.CorrelationID, "Detail data expires after 10 minutes. Re-run draw mode to capture fresh data.")}
	}

	result := map[string]any{
		"correlation_id":  detail.CorrelationID,
		"selector":        detail.Selector,
		"tag":             detail.Tag,
		"text_content":    detail.TextContent,
		"classes":         detail.Classes,
		"id":              detail.ID,
		"computed_styles": detail.ComputedStyles,
		"parent_selector": detail.ParentSelector,
		"bounding_rect":   detail.BoundingRect,
	}
	if len(detail.A11yFlags) > 0 {
		result["a11y_flags"] = detail.A11yFlags
	}
	if detail.OuterHTML != "" {
		result["outer_html"] = detail.OuterHTML
	}
	if len(detail.ShadowDOM) > 0 {
		result["shadow_dom"] = detail.ShadowDOM
	}
	if len(detail.AllElements) > 0 {
		result["all_elements"] = detail.AllElements
		result["element_count"] = detail.ElementCount
	}
	if len(detail.IframeContent) > 0 {
		result["iframe_content"] = detail.IframeContent
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Annotation detail", result)}
}
