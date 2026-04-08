// Purpose: Provides top-level interact dispatch, action routing, and jitter behavior.
// Why: Keeps orchestration logic centralized while action implementations live in focused files.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
	"math/rand/v2"
	"sort"
	"strings"
	"sync"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

// domPrimitiveActions delegates to the interact package.
var domPrimitiveActions = act.DOMPrimitiveActions

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
	PreDispatch: func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage, what string) (json.RawMessage, *JSONRPCResponse) {
		// Apply jitter before dispatch (moved here from handler wrapping to avoid concurrent map writes).
		h.interactAction().ApplyJitter(what)

		// Validate evidence mode.
		if _, err := toolinteract.ParseEvidenceMode(args); err != nil {
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

// interactHandlersOnce ensures the handler map is built exactly once, even under concurrency.
var interactHandlersOnce sync.Once

// cachedInteractHandlers is the lazily-initialized handler map for interact actions.
// Populated once via sync.Once on first call to getInteractHandlers() and reused thereafter.
var cachedInteractHandlers map[string]ModeHandler

// getInteractHandlers returns the cached unified handler map for interact actions.
// Merges both named handlers and DOM primitive actions into a single map[string]ModeHandler.
// The map is built once and cached for the process lifetime.
func getInteractHandlers() map[string]ModeHandler {
	interactHandlersOnce.Do(func() {
		cachedInteractHandlers = buildInteractHandlers()
	})
	return cachedInteractHandlers
}

// interactMethod adapts an InteractActionHandler method to a ModeHandler,
// removing the need for a one-line closure thunk per action.
func interactMethod(fn func(*toolinteract.InteractActionHandler, JSONRPCRequest, json.RawMessage) JSONRPCResponse) ModeHandler {
	return func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return fn(h.interactAction(), req, args)
	}
}

// stateMethod adapts a StateInteractHandler method to a ModeHandler.
func stateMethod(fn func(*toolinteract.StateInteractHandler, JSONRPCRequest, json.RawMessage) JSONRPCResponse) ModeHandler {
	return func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return fn(h.stateInteract(), req, args)
	}
}

// buildInteractHandlers constructs the full interact handler map.
func buildInteractHandlers() map[string]ModeHandler {
	handlers := map[string]ModeHandler{
		"highlight":              interactMethod((*toolinteract.InteractActionHandler).HandleHighlightImpl),
		"save_state":             stateMethod((*toolinteract.StateInteractHandler).HandleStateSave),
		"state_save":             stateMethod((*toolinteract.StateInteractHandler).HandleStateSave),
		"load_state":             stateMethod((*toolinteract.StateInteractHandler).HandleStateLoad),
		"state_load":             stateMethod((*toolinteract.StateInteractHandler).HandleStateLoad),
		"list_states":            stateMethod((*toolinteract.StateInteractHandler).HandleStateList),
		"state_list":             stateMethod((*toolinteract.StateInteractHandler).HandleStateList),
		"delete_state":           stateMethod((*toolinteract.StateInteractHandler).HandleStateDelete),
		"state_delete":           stateMethod((*toolinteract.StateInteractHandler).HandleStateDelete),
		"set_storage":            interactMethod((*toolinteract.InteractActionHandler).HandleSetStorage),
		"delete_storage":         interactMethod((*toolinteract.InteractActionHandler).HandleDeleteStorage),
		"clear_storage":          interactMethod((*toolinteract.InteractActionHandler).HandleClearStorage),
		"set_cookie":             interactMethod((*toolinteract.InteractActionHandler).HandleSetCookie),
		"delete_cookie":          interactMethod((*toolinteract.InteractActionHandler).HandleDeleteCookie),
		"execute_js":             interactMethod((*toolinteract.InteractActionHandler).HandleExecuteJSImpl),
		"navigate":               interactMethod((*toolinteract.InteractActionHandler).HandleBrowserActionNavigateImpl),
		"refresh":                interactMethod((*toolinteract.InteractActionHandler).HandleBrowserActionRefreshImpl),
		"back":                   interactMethod((*toolinteract.InteractActionHandler).HandleBrowserActionBackImpl),
		"forward":                interactMethod((*toolinteract.InteractActionHandler).HandleBrowserActionForwardImpl),
		"new_tab":                interactMethod((*toolinteract.InteractActionHandler).HandleBrowserActionNewTabImpl),
		"switch_tab":             interactMethod((*toolinteract.InteractActionHandler).HandleBrowserActionSwitchTabImpl),
		"close_tab":              interactMethod((*toolinteract.InteractActionHandler).HandleBrowserActionCloseTabImpl),
		"screenshot":             interactMethod((*toolinteract.InteractActionHandler).HandleScreenshotAliasImpl),
		"subtitle":               interactMethod((*toolinteract.InteractActionHandler).HandleSubtitleImpl),
		"list_interactive":       interactMethod((*toolinteract.InteractActionHandler).HandleListInteractive),
		"draw_mode_start":        interactMethod((*toolinteract.InteractActionHandler).HandleDrawModeStart),
		"hardware_click":         interactMethod((*toolinteract.InteractActionHandler).HandleHardwareClick),
		"activate_tab":           interactMethod((*toolinteract.InteractActionHandler).HandleActivateTabImpl),
		"get_readable":           interactMethod((*toolinteract.InteractActionHandler).HandleGetReadable),
		"get_markdown":           interactMethod((*toolinteract.InteractActionHandler).HandleGetMarkdown),
		"navigate_and_wait_for":  interactMethod((*toolinteract.InteractActionHandler).HandleNavigateAndWaitFor),
		"navigate_and_document":  interactMethod((*toolinteract.InteractActionHandler).HandleNavigateAndDocument),
		"fill_form_and_submit":   interactMethod((*toolinteract.InteractActionHandler).HandleFillFormAndSubmit),
		"fill_form":              interactMethod((*toolinteract.InteractActionHandler).HandleFillForm),
		"run_a11y_and_export_sarif": interactMethod((*toolinteract.InteractActionHandler).HandleRunA11yAndExportSARIF),
		"explore_page":           interactMethod((*toolinteract.InteractActionHandler).HandleExplorePage),
		"wait_for_stable":        interactMethod((*toolinteract.InteractActionHandler).HandleWaitForStable),
		"auto_dismiss_overlays":  interactMethod((*toolinteract.InteractActionHandler).HandleAutoDismissOverlays),
		"batch":                  interactMethod((*toolinteract.InteractActionHandler).HandleBatch),
		"clipboard_read":         interactMethod((*toolinteract.InteractActionHandler).HandleClipboardRead),
		"clipboard_write":        interactMethod((*toolinteract.InteractActionHandler).HandleClipboardWrite),

		// Recording and upload handlers access different sub-handlers on ToolHandler,
		// so they keep explicit closures.
		"screen_recording_start": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.recordingInteractHandler.handleRecordStart(req, args)
		},
		"record_start": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.recordingInteractHandler.handleRecordStart(req, args)
		},
		"screen_recording_stop": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.recordingInteractHandler.handleRecordStop(req, args)
		},
		"record_stop": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.recordingInteractHandler.handleRecordStop(req, args)
		},
		"upload": func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.uploadInteractHandler.HandleUpload(req, args)
		},
	}

	// Merge DOM primitive actions into the handler map.
	for action := range domPrimitiveActions {
		if _, exists := handlers[action]; exists {
			continue // named handler takes precedence (e.g. wait_for_stable, auto_dismiss_overlays)
		}
		action := action // capture for closure
		handlers[action] = func(th *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
			return th.interactAction().HandleDOMPrimitive(req, args, action)
		}
	}

	return handlers
}

// getValidInteractActions returns a sorted, comma-separated list of valid interact actions.
func getValidInteractActions() string {
	handlers := getInteractHandlers()
	sorted := make([]string, 0, len(handlers))
	for action := range handlers {
		sorted = append(sorted, action)
	}
	sort.Strings(sorted)
	return strings.Join(sorted, ", ")
}
