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
func (h *ToolHandler) interactDispatch() map[string]interactHandler {
	h.interactOnce.Do(func() {
		h.interactHandlers = map[string]interactHandler{
			"highlight":                 h.handleHighlight,
			"save_state":                h.handleStateSave,
			"state_save":                h.handleStateSave, // backward-compatible alias
			"load_state":                h.handleStateLoad,
			"state_load":                h.handleStateLoad, // backward-compatible alias
			"list_states":               h.handleStateList,
			"state_list":                h.handleStateList, // backward-compatible alias
			"delete_state":              h.handleStateDelete,
			"state_delete":              h.handleStateDelete, // backward-compatible alias
			"set_storage":               h.handleSetStorage,
			"delete_storage":            h.handleDeleteStorage,
			"clear_storage":             h.handleClearStorage,
			"set_cookie":                h.handleSetCookie,
			"delete_cookie":             h.handleDeleteCookie,
			"execute_js":                h.handleExecuteJS,
			"navigate":                  h.handleBrowserActionNavigate,
			"refresh":                   h.handleBrowserActionRefresh,
			"back":                      h.handleBrowserActionBack,
			"forward":                   h.handleBrowserActionForward,
			"new_tab":                   h.handleBrowserActionNewTab,
			"switch_tab":                h.handleBrowserActionSwitchTab,
			"close_tab":                 h.handleBrowserActionCloseTab,
			"screenshot":                h.handleScreenshotAlias,
			"subtitle":                  h.handleSubtitle,
			"list_interactive":          h.handleListInteractive,
			"record_start":              h.handleRecordStart,
			"record_stop":               h.handleRecordStop,
			"upload":                    h.handleUpload,
			"draw_mode_start":           h.handleDrawModeStart,
			"hardware_click":            h.handleHardwareClick,
			"activate_tab":              h.handleActivateTab,
			"get_readable":              h.handleGetReadable,
			"get_markdown":              h.handleGetMarkdown,
			"navigate_and_wait_for":     h.handleNavigateAndWaitFor,
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
	return h.interactHandlers
}

// getValidInteractActions returns a sorted, comma-separated list of valid interact actions.
func (h *ToolHandler) getValidInteractActions() string {
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
func (h *ToolHandler) applyJitter(action string) int {
	if readOnlyInteractActions[action] {
		return 0
	}
	h.jitterMu.RLock()
	maxMs := h.actionJitterMaxMs
	h.jitterMu.RUnlock()
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
func (h *ToolHandler) dispatchInteractAction(req JSONRPCRequest, args json.RawMessage, action string) JSONRPCResponse {
	h.applyJitter(action)
	if handler, ok := h.interactDispatch()[action]; ok {
		return handler(req, args)
	}
	if domPrimitiveActions[action] {
		return h.handleDOMPrimitive(req, args, action)
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown interact action: "+action, "Use a valid action from the 'what' enum", withParam("what"))}
}
