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

// interactAliasParams defines the deprecated alias parameters for the interact tool.
var interactAliasParams = []modeAlias{
	{JSONField: "action", DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
}

// interactRegistry is the tool registry for interact dispatch.
// PreDispatch handles evidence mode validation and async→background alias rewriting.
// PostDispatch handles composable side effects (subtitle, auto_dismiss, wait_for_stable,
// action_diff, include_screenshot, include_interactive).
var interactRegistry = toolRegistry{
	Handlers:  nil, // populated lazily per-call in toolInteract
	AliasDefs: interactAliasParams,
	Resolution: modeResolution{
		ToolName:   "interact",
		ValidModes: "", // populated lazily per-call in toolInteract
	},
	PreDispatch: func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage, _ string) (json.RawMessage, *JSONRPCResponse) {
		// Validate evidence mode.
		if _, err := parseEvidenceMode(args); err != nil {
			resp := fail(req, ErrInvalidParam,
				"Invalid 'evidence' value",
				"Use evidence='off' (default), 'on_mutation', or 'always'",
				withParam("evidence"))
			return args, &resp
		}
		// Quiet alias: async → background.
		args = mergeAsyncAlias(args)
		return args, nil
	},
	// PostDispatch is nil: composable side effects (subtitle, auto_dismiss, wait_for_stable,
	// action_diff, include_screenshot, include_interactive) are handled in toolInteract
	// after dispatchTool returns, since PostDispatch doesn't receive args.
}

// buildInteractHandlers returns the unified handler map for interact actions.
// Merges both named handlers and DOM primitive actions into a single map[string]ModeHandler.
func (h *interactActionHandler) buildInteractHandlers() map[string]ModeHandler {
	handlers := map[string]ModeHandler{
		"highlight": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleHighlightImpl(req, args)
		},
		"save_state": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.stateInteract().handleStateSave(req, args)
		},
		"state_save": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.stateInteract().handleStateSave(req, args)
		},
		"load_state": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.stateInteract().handleStateLoad(req, args)
		},
		"state_load": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.stateInteract().handleStateLoad(req, args)
		},
		"list_states": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.stateInteract().handleStateList(req, args)
		},
		"state_list": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.stateInteract().handleStateList(req, args)
		},
		"delete_state": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.stateInteract().handleStateDelete(req, args)
		},
		"state_delete": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.stateInteract().handleStateDelete(req, args)
		},
		"set_storage": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleSetStorage(req, args)
		},
		"delete_storage": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleDeleteStorage(req, args)
		},
		"clear_storage": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleClearStorage(req, args)
		},
		"set_cookie": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleSetCookie(req, args)
		},
		"delete_cookie": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleDeleteCookie(req, args)
		},
		"execute_js": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleExecuteJSImpl(req, args)
		},
		"navigate": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleBrowserActionNavigateImpl(req, args)
		},
		"refresh": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleBrowserActionRefreshImpl(req, args)
		},
		"back": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleBrowserActionBackImpl(req, args)
		},
		"forward": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleBrowserActionForwardImpl(req, args)
		},
		"new_tab": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleBrowserActionNewTabImpl(req, args)
		},
		"switch_tab": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleBrowserActionSwitchTabImpl(req, args)
		},
		"close_tab": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleBrowserActionCloseTabImpl(req, args)
		},
		"screenshot": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleScreenshotAliasImpl(req, args)
		},
		"subtitle": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleSubtitleImpl(req, args)
		},
		"list_interactive": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleListInteractive(req, args)
		},
		"screen_recording_start": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.recordingInteractHandler.handleRecordStart(req, args)
		},
		"screen_recording_stop": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.recordingInteractHandler.handleRecordStop(req, args)
		},
		"upload": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.uploadInteractHandler.handleUpload(req, args)
		},
		"draw_mode_start": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleDrawModeStart(req, args)
		},
		"hardware_click": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleHardwareClick(req, args)
		},
		"activate_tab": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleActivateTabImpl(req, args)
		},
		"get_readable": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleGetReadable(req, args)
		},
		"get_markdown": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleGetMarkdown(req, args)
		},
		"navigate_and_wait_for": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleNavigateAndWaitFor(req, args)
		},
		"navigate_and_document": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleNavigateAndDocument(req, args)
		},
		"fill_form_and_submit": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleFillFormAndSubmit(req, args)
		},
		"fill_form": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleFillForm(req, args)
		},
		"run_a11y_and_export_sarif": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleRunA11yAndExportSARIF(req, args)
		},
		"explore_page": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleExplorePage(req, args)
		},
		"wait_for_stable": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleWaitForStable(req, args)
		},
		"auto_dismiss_overlays": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleAutoDismissOverlays(req, args)
		},
		"batch": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleBatch(req, args)
		},
		"clipboard_read": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleClipboardRead(req, args)
		},
		"clipboard_write": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleClipboardWrite(req, args)
		},
	}

	// Merge DOM primitive actions into the handler map.
	for action := range domPrimitiveActions {
		if _, exists := handlers[action]; exists {
			continue // named handler takes precedence (e.g. wait_for_stable, auto_dismiss_overlays)
		}
		action := action // capture for closure
		handlers[action] = func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().handleDOMPrimitive(req, args, action)
		}
	}

	return handlers
}


// getValidInteractActions returns a sorted, comma-separated list of valid interact actions.
func (h *interactActionHandler) getValidInteractActions() string {
	handlers := h.buildInteractHandlers()
	sorted := make([]string, 0, len(handlers))
	for action := range handlers {
		sorted = append(sorted, action)
	}
	sort.Strings(sorted)
	return strings.Join(sorted, ", ")
}

// readOnlyInteractActions lists actions that should not have jitter applied.
// SYNC: The TS source of truth is src/background/action-metadata.ts (ACTION_METADATA map).
// When adding or reclassifying actions, update both this map and the TS metadata.
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

