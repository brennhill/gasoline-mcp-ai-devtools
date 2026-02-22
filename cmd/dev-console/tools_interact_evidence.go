// tools_interact_evidence.go — Evidence capture state machine for interact actions.
//
// Evidence mode is opt-in per interact call via:
//
//	evidence: "off" | "on_mutation" | "always"
//
// When enabled, the server captures screenshot artifacts before and after actions
// and surfaces them in command results as evidence.before/evidence.after.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
)

type evidenceMode string

const (
	evidenceModeOff        evidenceMode = "off"
	evidenceModeOnMutation evidenceMode = "on_mutation"
	evidenceModeAlways     evidenceMode = "always"
)

const (
	evidenceRetryEnv       = "GASOLINE_EVIDENCE_RETRY_COUNT"
	evidenceMaxCapturesEnv = "GASOLINE_EVIDENCE_MAX_CAPTURES_PER_COMMAND"
)

type evidenceShot struct {
	Path     string
	Filename string
	Error    string
	Attempts int
}

type commandEvidenceState struct {
	mode          evidenceMode
	action        string
	shouldCapture bool
	maxCaptures   int
	clientID      string
	skipped       string

	before evidenceShot
	after  evidenceShot

	finalized bool
	cached    map[string]any
}

var evidenceCaptureFn = defaultEvidenceCapture

func parseEvidenceMode(args json.RawMessage) (evidenceMode, error) {
	var params struct {
		Evidence string `json:"evidence"`
	}
	lenientUnmarshal(args, &params)
	raw := strings.TrimSpace(params.Evidence)
	if raw == "" {
		return evidenceModeOff, nil
	}

	mode := evidenceMode(strings.ToLower(raw))
	switch mode {
	case evidenceModeOff, evidenceModeOnMutation, evidenceModeAlways:
		return mode, nil
	default:
		return evidenceModeOff, fmt.Errorf("invalid evidence mode: %s", raw)
	}
}

func evidenceMaxCapturesPerCommand() int {
	return parseBoundedEnvInt(evidenceMaxCapturesEnv, 2, 0, 2)
}

func evidenceRetryCount() int {
	return parseBoundedEnvInt(evidenceRetryEnv, 1, 0, 3)
}

func parseBoundedEnvInt(name string, def, min, max int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func defaultEvidenceCapture(h *ToolHandler, clientID string) evidenceShot {
	if h == nil || h.capture == nil {
		return evidenceShot{Error: "capture_not_initialized"}
	}
	enabled, _, _ := h.capture.GetTrackingStatus()
	if !enabled {
		return evidenceShot{Error: "no_tracked_tab"}
	}

	queryID := h.capture.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "screenshot",
			Params: json.RawMessage(`{}`),
		},
		12*time.Second,
		clientID,
	)

	raw, err := h.capture.WaitForResult(queryID, 12*time.Second)
	if err != nil {
		return evidenceShot{Error: "screenshot_timeout: " + err.Error()}
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return evidenceShot{Error: "screenshot_parse_error: " + err.Error()}
	}

	if errMsg, ok := payload["error"].(string); ok && strings.TrimSpace(errMsg) != "" {
		return evidenceShot{Error: strings.TrimSpace(errMsg)}
	}

	path, _ := payload["path"].(string)
	filename, _ := payload["filename"].(string)
	path = strings.TrimSpace(path)
	filename = strings.TrimSpace(filename)
	if path == "" {
		return evidenceShot{
			Filename: filename,
			Error:    "screenshot_missing_path",
		}
	}

	return evidenceShot{
		Path:     path,
		Filename: filename,
	}
}

func canonicalActionFromInteractArgs(args json.RawMessage) string {
	var params struct {
		What   string `json:"what"`
		Action string `json:"action"`
	}
	lenientUnmarshal(args, &params)
	action := strings.TrimSpace(params.What)
	if action == "" {
		action = strings.TrimSpace(params.Action)
	}
	return strings.ToLower(action)
}

func isMutationAction(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case
		"highlight",
		"execute_js",
		"navigate", "refresh", "back", "forward", "new_tab", "switch_tab", "close_tab",
		"click", "type", "select", "check", "paste", "key_press",
		"set_attribute", "scroll_to", "focus",
		"open_composer", "submit_active_composer", "confirm_top_dialog", "dismiss_top_overlay",
		"set_storage", "delete_storage", "clear_storage",
		"set_cookie", "delete_cookie",
		"fill_form", "fill_form_and_submit",
		"upload":
		return true
	default:
		return false
	}
}

func (h *ToolHandler) captureEvidenceWithRetry(clientID string) evidenceShot {
	retries := evidenceRetryCount()
	attempts := retries + 1
	last := evidenceShot{Error: "evidence_capture_not_attempted"}

	for i := 0; i < attempts; i++ {
		shot := evidenceCaptureFn(h, clientID)
		shot.Attempts = i + 1
		if strings.TrimSpace(shot.Path) != "" {
			return shot
		}
		if strings.TrimSpace(shot.Error) == "" {
			shot.Error = "evidence_capture_failed"
		}
		last = shot
		if i < attempts-1 {
			time.Sleep(150 * time.Millisecond)
		}
	}

	return last
}

func (h *ToolHandler) armEvidenceForCommand(correlationID, action string, args json.RawMessage, clientID string) {
	if h == nil || correlationID == "" {
		return
	}

	h.armRetryContract(correlationID, action, args)

	mode, err := parseEvidenceMode(args)
	if err != nil {
		return
	}

	if mode == evidenceModeOff {
		h.evidenceMu.Lock()
		delete(h.evidenceByCommand, correlationID)
		h.evidenceMu.Unlock()
		return
	}

	if action == "" {
		action = canonicalActionFromInteractArgs(args)
	}

	maxCaptures := evidenceMaxCapturesPerCommand()
	state := &commandEvidenceState{
		mode:        mode,
		action:      strings.ToLower(strings.TrimSpace(action)),
		maxCaptures: maxCaptures,
		clientID:    clientID,
	}

	switch mode {
	case evidenceModeAlways:
		state.shouldCapture = true
	case evidenceModeOnMutation:
		state.shouldCapture = isMutationAction(state.action)
		if !state.shouldCapture {
			state.skipped = "non_mutating_action"
		}
	}

	if state.shouldCapture && state.maxCaptures <= 0 {
		state.shouldCapture = false
		state.skipped = "capture_budget_zero"
	}

	if state.shouldCapture {
		state.before = h.captureEvidenceWithRetry(clientID)
	}

	h.evidenceMu.Lock()
	if h.evidenceByCommand == nil {
		h.evidenceByCommand = make(map[string]*commandEvidenceState)
	}
	h.evidenceByCommand[correlationID] = state
	h.evidenceMu.Unlock()
}

func (h *ToolHandler) attachEvidencePayload(correlationID string, responseData map[string]any) {
	if h == nil || correlationID == "" || responseData == nil {
		return
	}

	h.evidenceMu.Lock()
	state, ok := h.evidenceByCommand[correlationID]
	if !ok {
		h.evidenceMu.Unlock()
		return
	}
	if state.finalized {
		responseData["evidence"] = cloneAnyMap(state.cached)
		h.evidenceMu.Unlock()
		return
	}
	needsAfter := state.shouldCapture && state.maxCaptures > 1
	clientID := state.clientID
	h.evidenceMu.Unlock()

	var after evidenceShot
	if needsAfter {
		after = h.captureEvidenceWithRetry(clientID)
	}

	h.evidenceMu.Lock()
	state, ok = h.evidenceByCommand[correlationID]
	if !ok {
		h.evidenceMu.Unlock()
		return
	}
	if !state.finalized {
		if needsAfter {
			state.after = after
		}
		state.cached = buildEvidencePayload(state)
		state.finalized = true
	}
	payload := cloneAnyMap(state.cached)
	h.evidenceMu.Unlock()

	responseData["evidence"] = payload
}

func buildEvidencePayload(state *commandEvidenceState) map[string]any {
	if state == nil {
		return map[string]any{}
	}

	payload := map[string]any{
		"mode":   string(state.mode),
		"action": state.action,
	}

	if state.before.Path != "" {
		payload["before"] = state.before.Path
	}
	if state.after.Path != "" {
		payload["after"] = state.after.Path
	}

	files := map[string]any{}
	if state.before.Filename != "" {
		files["before"] = state.before.Filename
	}
	if state.after.Filename != "" {
		files["after"] = state.after.Filename
	}
	if len(files) > 0 {
		payload["filenames"] = files
	}

	errors := map[string]any{}
	if state.before.Error != "" {
		errors["before"] = state.before.Error
	}
	if state.after.Error != "" {
		errors["after"] = state.after.Error
	}
	if len(errors) > 0 {
		payload["errors"] = errors
	}

	if state.skipped != "" {
		payload["skipped"] = state.skipped
	}

	if len(errors) > 0 && (state.before.Path != "" || state.after.Path != "") {
		payload["partial"] = true
	}

	return payload
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		if nested, ok := v.(map[string]any); ok {
			out[k] = cloneAnyMap(nested)
			continue
		}
		out[k] = v
	}
	return out
}
