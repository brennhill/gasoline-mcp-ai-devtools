// Purpose: Provides top-level interact dispatch, action routing, and jitter behavior.
// Why: Keeps orchestration logic centralized while action implementations live in focused files.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
	"math/rand/v2"
	"sort"
	"strings"
	"time"
)

// randIntn returns a random int in [0, n). Uses math/rand/v2 which auto-seeds.
func randIntn(n int) int {
	if n <= 0 {
		return 0
	}
	return rand.IntN(n)
}

// interactHandler is the function signature for interact action handlers.
type interactHandler func(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

// interactDispatch returns the dispatch map for interact actions.
// Initialized once via sync.Once; safe for concurrent use.
func (h *interactActionHandler) interactDispatch() map[string]interactHandler {
	h.once.Do(func() {
		h.handlers = map[string]interactHandler{
			"highlight":                 h.handleHighlightImpl,
			"save_state":                h.parent.stateInteract().handleStateSave,
			"state_save":                h.parent.stateInteract().handleStateSave, // backward-compatible alias
			"load_state":                h.parent.stateInteract().handleStateLoad,
			"state_load":                h.parent.stateInteract().handleStateLoad, // backward-compatible alias
			"list_states":               h.parent.stateInteract().handleStateList,
			"state_list":                h.parent.stateInteract().handleStateList, // backward-compatible alias
			"delete_state":              h.parent.stateInteract().handleStateDelete,
			"state_delete":              h.parent.stateInteract().handleStateDelete, // backward-compatible alias
			"set_storage":               h.handleSetStorage,
			"delete_storage":            h.handleDeleteStorage,
			"clear_storage":             h.handleClearStorage,
			"set_cookie":                h.handleSetCookie,
			"delete_cookie":             h.handleDeleteCookie,
			"execute_js":                h.handleExecuteJSImpl,
			"navigate":                  h.handleBrowserActionNavigateImpl,
			"refresh":                   h.handleBrowserActionRefreshImpl,
			"back":                      h.handleBrowserActionBackImpl,
			"forward":                   h.handleBrowserActionForwardImpl,
			"new_tab":                   h.handleBrowserActionNewTabImpl,
			"switch_tab":                h.handleBrowserActionSwitchTabImpl,
			"close_tab":                 h.handleBrowserActionCloseTabImpl,
			"screenshot":                h.handleScreenshotAliasImpl,
			"subtitle":                  h.handleSubtitleImpl,
			"list_interactive":          h.handleListInteractive,
			"screen_recording_start":    h.parent.recordingInteractHandler.handleRecordStart,
			"screen_recording_stop":     h.parent.recordingInteractHandler.handleRecordStop,
			"upload":                    h.parent.uploadInteractHandler.handleUpload,
			"draw_mode_start":           h.handleDrawModeStart,
			"hardware_click":            h.handleHardwareClick,
			"activate_tab":              h.handleActivateTabImpl,
			"get_readable":              h.handleGetReadable,
			"get_markdown":              h.handleGetMarkdown,
			"navigate_and_wait_for":     h.handleNavigateAndWaitFor,
			"navigate_and_document":     h.handleNavigateAndDocument,
			"fill_form_and_submit":      h.handleFillFormAndSubmit,
			"fill_form":                 h.handleFillForm,
			"run_a11y_and_export_sarif": h.handleRunA11yAndExportSARIF,
			"explore_page":              h.handleExplorePage,
			"wait_for_stable":           h.handleWaitForStable,
			"auto_dismiss_overlays":     h.handleAutoDismissOverlays,
			"batch":                     h.handleBatch,
			"clipboard_read":            h.handleClipboardRead,
			"clipboard_write":           h.handleClipboardWrite,
		}
	})
	return h.handlers
}

// getValidInteractActions returns a sorted, comma-separated list of valid interact actions.
func (h *interactActionHandler) getValidInteractActions() string {
	actions := make(map[string]bool)
	for action := range h.interactDispatch() {
		actions[action] = true
	}
	for action := range domPrimitiveActions {
		actions[action] = true
	}
	sorted := make([]string, 0, len(actions))
	for action := range actions {
		sorted = append(sorted, action)
	}
	sort.Strings(sorted)
	return strings.Join(sorted, ", ")
}

// readOnlyInteractActions lists actions that should not have jitter applied.
var readOnlyInteractActions = map[string]bool{
	"list_interactive":          true,
	"get_text":                  true,
	"get_value":                 true,
	"get_attribute":             true,
	"query":                     true,
	"screenshot":                true,
	"list_states":               true,
	"state_list":                true,
	"get_readable":              true,
	"get_markdown":              true,
	"explore_page":              true,
	"run_a11y_and_export_sarif": true,
	"wait_for":                  true,
	"wait_for_stable":           true,
	"auto_dismiss_overlays":     true,
	"batch":                     true,
	"highlight":                 true,
	"subtitle":                  true,
	"clipboard_read":            true,
}

// applyJitter sleeps for a random duration up to maxMs if jitter is configured.
func (h *interactActionHandler) applyJitter(action string) int {
	if readOnlyInteractActions[action] {
		return 0
	}
	h.parent.jitterMu.RLock()
	maxMs := h.parent.actionJitterMaxMs
	h.parent.jitterMu.RUnlock()
	if maxMs <= 0 {
		return 0
	}
	jitterMs := randIntn(maxMs)
	if jitterMs > 0 {
		time.Sleep(time.Duration(jitterMs) * time.Millisecond)
	}
	return jitterMs
}

// dispatchInteractAction routes an action to the correct named or DOM-primitive handler.
func (h *interactActionHandler) dispatchInteractAction(req JSONRPCRequest, args json.RawMessage, action string) JSONRPCResponse {
	h.applyJitter(action)
	if handler, ok := h.interactDispatch()[action]; ok {
		return handler(req, args)
	}
	if domPrimitiveActions[action] {
		return h.handleDOMPrimitive(req, args, action)
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown interact action: "+action, "Use a valid action from the 'what' enum", withParam("what"))}
}
