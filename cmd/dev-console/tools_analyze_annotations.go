// tools_analyze_annotations.go — Analyze handlers for draw mode annotations.
// Provides analyze({what: "annotations"}), analyze({what: "annotation_detail"}),
// analyze({what: "draw_history"}), and analyze({what: "draw_session"}).
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// annotationWaitCommandTTL is how long the pending command stays in CommandTracker
// before being expired. Generous so the user has time to draw.
const annotationWaitCommandTTL = 10 * time.Minute

// toolGetAnnotations returns the latest annotation session or a named multi-page session.
// If wait=true, returns immediately with a correlation_id. The LLM polls via
// observe({what: "command_result", correlation_id: "..."}) until annotations arrive.
// If session is specified, returns the named multi-page session.
func (h *ToolHandler) toolGetAnnotations(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Wait    bool   `json:"wait"`
		Session string `json:"session"`
	}
	if len(args) > 0 {
		lenientUnmarshal(args, &params)
	}

	// Named session path — returns multi-page aggregated results
	if params.Session != "" {
		return h.getNamedAnnotations(req, params.Session, params.Wait)
	}

	// Anonymous session path — returns latest single-page result
	return h.getAnonymousAnnotations(req, params.Wait)
}

func (h *ToolHandler) getAnonymousAnnotations(req JSONRPCRequest, wait bool) JSONRPCResponse {
	if wait {
		// Check if annotations are already available (no need to wait)
		if session := h.annotationStore.GetLatestSessionSinceDraw(); session != nil {
			return h.formatAnnotationSession(req, session)
		}

		// Register a pending command — the annotation store will complete it
		// when annotations arrive via /draw-mode/complete
		corrID := fmt.Sprintf("ann_%d_%d", time.Now().UnixNano(), randomInt63()%1000000)
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
	result := map[string]any{
		"annotations": session.Annotations,
		"count":       len(session.Annotations),
		"page_url":    session.PageURL,
	}
	if session.ScreenshotPath != "" {
		result["screenshot"] = session.ScreenshotPath
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Annotations retrieved", result)}
}

// #lizard forgives
func (h *ToolHandler) getNamedAnnotations(req JSONRPCRequest, sessionName string, wait bool) JSONRPCResponse {
	if wait {
		// Check if named session already has data newer than draw start
		if ns := h.annotationStore.GetNamedSessionSinceDraw(sessionName); ns != nil {
			return h.formatNamedAnnotationSession(req, ns)
		}

		corrID := fmt.Sprintf("ann_%d_%d", time.Now().UnixNano(), randomInt63()%1000000)
		h.capture.RegisterCommand(corrID, "", annotationWaitCommandTTL)
		h.annotationStore.RegisterWaiter(corrID, sessionName)

		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Waiting for annotations", map[string]any{
			"status":         "waiting_for_user",
			"correlation_id": corrID,
			"session_name":   sessionName,
			"pages":          []any{},
			"page_count":     0,
			"total_count":    0,
			"message":        "Draw mode is active. The user is drawing annotations. Poll with observe({what: 'command_result', correlation_id: '" + corrID + "'}) to check for results.",
		})}
	}

	ns := h.annotationStore.GetNamedSession(sessionName)
	if ns == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("No annotations", map[string]any{
			"session_name": sessionName,
			"pages":        []any{},
			"page_count":   0,
			"total_count":  0,
			"message":      "Named session '" + sessionName + "' not found. Use interact({action: 'draw_mode_start', session: '" + sessionName + "'}) to start.",
		})}
	}

	return h.formatNamedAnnotationSession(req, ns)
}

func (h *ToolHandler) formatNamedAnnotationSession(req JSONRPCRequest, ns *NamedAnnotationSession) JSONRPCResponse {
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

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Annotations retrieved", map[string]any{
		"session_name": ns.Name,
		"pages":        pages,
		"page_count":   len(ns.Pages),
		"total_count":  totalCount,
	})}
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

// toolListDrawHistory lists all persisted draw session files from disk.
func (h *ToolHandler) toolListDrawHistory(req JSONRPCRequest, _ json.RawMessage) JSONRPCResponse {
	dir, err := screenshotsDir()
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "Cannot access screenshots directory: "+err.Error(), "Check file permissions")}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "Cannot read screenshots directory: "+err.Error(), "Check file permissions")}
	}

	type sessionSummary struct {
		File      string `json:"file"`
		Path      string `json:"path"`
		SizeBytes int64  `json:"size_bytes"`
		ModTime   int64  `json:"mod_time"`
	}

	sessions := make([]sessionSummary, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "draw-session-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		sessions = append(sessions, sessionSummary{
			File:      entry.Name(),
			Path:      filepath.Join(dir, entry.Name()),
			SizeBytes: info.Size(),
			ModTime:   info.ModTime().UnixMilli(),
		})
	}

	// Sort newest first
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModTime > sessions[j].ModTime
	})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Draw session history", map[string]any{
		"sessions":   sessions,
		"count":      len(sessions),
		"storage_dir": dir,
	})}
}

// toolGetDrawSession reads a specific draw session file from disk.
func (h *ToolHandler) toolGetDrawSession(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		File string `json:"file"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.File == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'file' is missing", "Provide the session filename from draw_history results", withParam("file"))}
	}

	// Validate filename to prevent path traversal
	if strings.Contains(params.File, "/") || strings.Contains(params.File, "\\") || strings.Contains(params.File, "..") {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Invalid filename: path traversal not allowed", "Use only the filename from draw_history results", withParam("file"))}
	}

	dir, err := screenshotsDir()
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "Cannot access screenshots directory: "+err.Error(), "Check file permissions")}
	}

	path := filepath.Join(dir, params.File)
	if !isWithinDir(path, dir) {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Invalid filename: resolved path outside screenshots directory", "Use only the filename from draw_history results", withParam("file"))}
	}
	data, err := os.ReadFile(path) // #nosec G304 -- filename validated against path traversal above
	if err != nil {
		if os.IsNotExist(err) {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "Draw session file not found: "+params.File, "Use analyze({what:'draw_history'}) to list available sessions")}
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "Cannot read draw session file: "+err.Error(), "Check file permissions")}
	}

	var session map[string]any
	if err := json.Unmarshal(data, &session); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Corrupted draw session file: "+err.Error(), "The file may be damaged. Try a different session.")}
	}

	session["_file"] = params.File
	session["_path"] = path

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Draw session loaded", session)}
}
