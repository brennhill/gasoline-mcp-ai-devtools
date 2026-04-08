// Purpose: Thin dispatch methods for annotation retrieval — delegates to toolanalyze.
// Why: Keeps annotation business logic in toolanalyze; this file handles MCP plumbing only.
// Docs: docs/features/feature/annotated-screenshots/index.md

package main

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolanalyze"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
)

// toolGetAnnotations returns latest annotation session or a named multi-page session.
func (h *ToolHandler) toolGetAnnotations(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Wait         *bool  `json:"wait"`
		Background   *bool  `json:"background"`
		AnnotSession string `json:"annot_session"`
		Operation    string `json:"operation"`
		Correlation  string `json:"correlation_id"`
		TimeoutMs    int    `json:"timeout_ms"`
		URL          string `json:"url"`
		URLPattern   string `json:"url_pattern"`
	}
	if len(args) > 0 {
		lenientUnmarshal(args, &params)
	}
	// Canonical param is "background" (false = block). "wait" is a quiet alias.
	// Default to wait=true when both background and wait are omitted.
	waitValue := true // default: blocking
	if params.Background != nil {
		waitValue = !*params.Background
	} else if params.Wait != nil {
		waitValue = *params.Wait
	}

	urlFilter, errMsg, hasFilterErr := toolanalyze.ResolveAnnotationURLFilter(params.URL, params.URLPattern)
	if hasFilterErr {
		return fail(req, ErrInvalidParam, errMsg,
			"Provide only one annotation scope filter, or set both to the same value.",
			withParam("url"), withParam("url_pattern"))
	}

	operation := strings.ToLower(strings.TrimSpace(params.Operation))
	if operation != "" {
		if operation != "flush" {
			return fail(req, ErrInvalidParam, "Invalid annotations operation: "+params.Operation, "Use operation='flush' for annotation waiter recovery.", withParam("operation"), withHint("flush"))
		}
		return h.toolFlushAnnotations(req, params.Correlation, urlFilter)
	}

	waitTimeout := toolanalyze.AnnotationBlockingWaitDuration(params.TimeoutMs)
	if params.AnnotSession != "" {
		return h.getNamedAnnotations(req, params.AnnotSession, waitValue, waitTimeout, urlFilter)
	}
	return h.getAnonymousAnnotations(req, waitValue, waitTimeout, urlFilter)
}

func (h *ToolHandler) getAnonymousAnnotations(req JSONRPCRequest, wait bool, waitTimeout time.Duration, urlFilter string) JSONRPCResponse {
	if wait {
		if session := h.annotationStore.GetLatestSessionSinceDraw(); session != nil {
			return succeed(req, "Annotations retrieved", toolanalyze.BuildAnnotationSessionResult(session, urlFilter))
		}

		if session, _ := h.annotationStore.WaitForSession(waitTimeout); session != nil {
			return succeed(req, "Annotations retrieved", toolanalyze.BuildAnnotationSessionResult(session, urlFilter))
		}

		corrID := newCorrelationID("ann")
		h.capture.RegisterCommand(corrID, "", toolanalyze.AnnotationWaitCommandTTL)
		h.annotationStore.RegisterWaiter(corrID, "", urlFilter)

		return succeed(req, "Waiting for annotations", map[string]any{
			"status":         "waiting_for_user",
			"correlation_id": corrID,
			"annotations":    []any{},
			"count":          0,
			"filter_applied": annotation.FilterAppliedValue(urlFilter),
			"message":        "Draw mode is active. The user is drawing annotations. Poll with observe({what: 'command_result', correlation_id: '" + corrID + "'}) to check for results.",
		})
	}

	session := h.annotationStore.GetLatestSession()
	if session == nil {
		return succeed(req, "No annotations", map[string]any{
			"annotations":    []any{},
			"count":          0,
			"filter_applied": annotation.FilterAppliedValue(urlFilter),
			"message":        "No annotation session found. Use interact({action: 'draw_mode_start'}) to activate draw mode, then the user draws annotations and presses ESC to finish.",
		})
	}
	return succeed(req, "Annotations retrieved", toolanalyze.BuildAnnotationSessionResult(session, urlFilter))
}

// #lizard forgives
func (h *ToolHandler) getNamedAnnotations(req JSONRPCRequest, sessionName string, wait bool, waitTimeout time.Duration, urlFilter string) JSONRPCResponse {
	if wait {
		if ns := h.annotationStore.GetNamedSessionSinceDraw(sessionName); ns != nil {
			return succeed(req, "Annotations retrieved", toolanalyze.BuildNamedAnnotationSessionResult(ns, urlFilter))
		}

		if ns, _ := h.annotationStore.WaitForNamedSession(sessionName, waitTimeout); ns != nil {
			return succeed(req, "Annotations retrieved", toolanalyze.BuildNamedAnnotationSessionResult(ns, urlFilter))
		}

		corrID := newCorrelationID("ann")
		h.capture.RegisterCommand(corrID, "", toolanalyze.AnnotationWaitCommandTTL)
		h.annotationStore.RegisterWaiter(corrID, sessionName, urlFilter)

		return succeed(req, "Waiting for annotations", map[string]any{
			"status":             "waiting_for_user",
			"correlation_id":     corrID,
			"annot_session_name": sessionName,
			"pages":              []any{},
			"page_count":         0,
			"total_count":        0,
			"filter_applied":     annotation.FilterAppliedValue(urlFilter),
			"message":            "Draw mode is active. The user is drawing annotations. Poll with observe({what: 'command_result', correlation_id: '" + corrID + "'}) to check for results.",
		})
	}

	ns := h.annotationStore.GetNamedSession(sessionName)
	if ns == nil {
		return succeed(req, "No annotations", map[string]any{
			"annot_session_name": sessionName,
			"pages":              []any{},
			"page_count":         0,
			"total_count":        0,
			"filter_applied":     annotation.FilterAppliedValue(urlFilter),
			"message":            "Named session '" + sessionName + "' not found. Use interact({action: 'draw_mode_start', annot_session: '" + sessionName + "'}) to start.",
		})
	}

	return succeed(req, "Annotations retrieved", toolanalyze.BuildNamedAnnotationSessionResult(ns, urlFilter))
}

// toolFlushAnnotations forces completion of a pending annotation waiter.
// This is a recovery path for stuck waiters that would otherwise remain pending.
func (h *ToolHandler) toolFlushAnnotations(req JSONRPCRequest, correlationID string, fallbackURLFilter string) JSONRPCResponse {
	correlationID = strings.TrimSpace(correlationID)
	if correlationID == "" {
		return fail(req, ErrMissingParam,
			"Required parameter 'correlation_id' is missing for operation='flush'",
			"Pass the correlation_id returned by analyze({what:'annotations',wait:true}).",
			withParam("correlation_id"))
	}
	if !strings.HasPrefix(correlationID, "ann_") {
		return fail(req, ErrInvalidParam,
			"Invalid annotation correlation_id: "+correlationID,
			"Use an annotation correlation_id (prefix ann_) from analyze({what:'annotations',wait:true}).",
			withParam("correlation_id"))
	}

	// Remove waiter first so later session writes cannot re-complete the same flush target.
	sessionName, waiterURLFilter, _ := h.annotationStore.TakeWaiter(correlationID)

	// Idempotent behavior: if the command is already terminal, return current state.
	if cmd, found := h.capture.GetCommandResult(correlationID); found && cmd != nil && cmd.Status != "pending" {
		return h.formatCommandResult(req, *cmd, correlationID)
	}

	effectiveURLFilter := waiterURLFilter
	if strings.TrimSpace(effectiveURLFilter) == "" {
		effectiveURLFilter = fallbackURLFilter
	}

	flushData := toolanalyze.BuildFlushedAnnotationResult(h.annotationStore, sessionName, effectiveURLFilter)
	payload, _ := json.Marshal(flushData)
	h.capture.ApplyCommandResult(correlationID, "complete", payload, "")

	// Normal path: return canonical command_result envelope.
	if cmd, found := h.capture.GetCommandResult(correlationID); found && cmd != nil {
		return h.formatCommandResult(req, *cmd, correlationID)
	}

	// Recovery fallback: command tracker no longer has this correlation_id.
	flushData["status"] = "complete"
	flushData["final"] = true
	flushData["correlation_id"] = correlationID
	flushData["lifecycle_status"] = "complete"
	return succeed(req, "Annotation flush completed", flushData)
}

// toolGetAnnotationDetail returns full DOM/style detail for a specific annotation.
func (h *ToolHandler) toolGetAnnotationDetail(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		CorrelationID string `json:"correlation_id"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	if resp, blocked := requireString(req, params.CorrelationID, "correlation_id", "Add the 'correlation_id' from the annotation you want detail for"); blocked {
		return resp
	}

	detail, found := h.annotationStore.GetDetail(params.CorrelationID)
	if !found {
		return fail(req, ErrNoData, "Annotation detail not found or expired for correlation_id: "+params.CorrelationID, "Detail data expires after 10 minutes. Re-run draw mode to capture fresh data.")
	}

	// Gather correlated errors for context
	var correlatedErrors []map[string]string
	if annotTS := h.annotationStore.FindAnnotationTimestamp(params.CorrelationID); annotTS > 0 {
		correlatedErrors = h.findErrorsNearTimestamp(annotTS, toolanalyze.AnnotationErrorCorrelationWindow)
	}

	result := toolanalyze.BuildAnnotationDetailResult(detail, correlatedErrors)
	return succeed(req, "Annotation detail", result)
}
