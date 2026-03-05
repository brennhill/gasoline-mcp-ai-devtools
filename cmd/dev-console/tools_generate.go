// Purpose: Dispatches generate tool modes (reproduction, test, pr_summary, sarif, har, csp, sri, visual_test, annotation_report, annotation_issues, test_from_context, test_heal, test_classify) and assembles output artifacts.
// Why: Acts as the top-level router for all artifact generation, delegating format-specific logic to sub-handlers.
// Docs: docs/features/feature/test-generation/index.md
package main

import (
	"encoding/json"

	gen "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/tools/generate"
)

// GenerateHandler is the function signature for generate format handlers.
type GenerateHandler func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

// generateHandlers maps generate format names to their handler functions.
var generateHandlers = map[string]GenerateHandler{
	"reproduction": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGetReproductionScript(req, args)
	},
	"test": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGenerateTest(req, args)
	},
	"pr_summary": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGeneratePRSummary(req, args)
	},
	"sarif": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolExportSARIF(req, args)
	},
	"har": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolExportHAR(req, args)
	},
	"csp": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGenerateCSP(req, args)
	},
	"sri": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGenerateSRI(req, args)
	},
	"test_from_context": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.testGen().handleGenerateTestFromContext(req, args)
	},
	"test_heal": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.testGen().handleGenerateTestHeal(req, args)
	},
	"test_classify": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.testGen().handleGenerateTestClassify(req, args)
	},
	"visual_test": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGenerateVisualTest(req, args)
	},
	"annotation_report": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGenerateAnnotationReport(req, args)
	},
	"annotation_issues": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGenerateAnnotationIssues(req, args)
	},
}

// isGenerateMode returns true when the value is a known top-level generate mode.
func isGenerateMode(v string) bool {
	_, ok := generateHandlers[v]
	return ok
}

// generateAliasParams defines the deprecated alias parameters for the generate tool.
// "action" is only treated as a mode alias when its value matches a known generate mode,
// since "action" can also be a sub-action parameter (e.g. test_heal action=analyze).
// Both ConflictFn and FallbackFn are gated to handler membership.
var generateAliasParams = []modeAlias{
	{JSONField: "format"},
	{JSONField: "action", ConflictFn: isGenerateMode, FallbackFn: isGenerateMode},
}

// getValidGenerateFormats returns a sorted, comma-separated list of valid generate formats.
func getValidGenerateFormats() string { return sortedMapKeys(generateHandlers) }

// toolGenerate dispatches generate requests based on the 'what' parameter.
func (h *ToolHandler) toolGenerate(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	what, usedAliasParam, errResp := resolveToolMode(req, args, generateAliasParams, modeResolution{
		ToolName:   "generate",
		ValidModes: getValidGenerateFormats(),
	})
	if errResp != nil {
		return *errResp
	}

	handler, ok := generateHandlers[what]
	if !ok {
		validFormats := getValidGenerateFormats()
		resp := fail(req, ErrUnknownMode, "Unknown generate format: "+what, "Use a valid format from the 'what' enum", withParam("what"), withHint("Valid values: "+validFormats), describeCapabilitiesRecovery("generate"))
		return appendCanonicalWhatAliasWarning(resp, usedAliasParam, what)
	}

	// Strict parameter validation: reject unknown params for the given format
	if errResp := validateGenerateParams(req, what, args); errResp != nil {
		return appendCanonicalWhatAliasWarning(*errResp, usedAliasParam, what)
	}

	resp := handler(h, req, args)
	return appendCanonicalWhatAliasWarning(resp, usedAliasParam, what)
}

// ============================================
// Generate sub-handlers
// ============================================

func (h *ToolHandler) toolGetReproductionScript(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.toolGetReproductionScriptImpl(req, args)
}

// TestGenParams delegates to internal/tools/generate.
type TestGenParams = gen.TestGenParams

func (h *ToolHandler) toolGenerateTest(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.generateTestImpl(req, args)
}

func (h *ToolHandler) toolGeneratePRSummary(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.generatePRSummaryImpl(req, args)
}

func (h *ToolHandler) toolExportSARIF(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.exportSARIFImpl(req, args)
}

func (h *ToolHandler) toolExportHAR(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.exportHARImpl(req, args)
}

func (h *ToolHandler) toolGenerateCSP(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.generateCSPImpl(req, args)
}

// toolGenerateSRI generates Subresource Integrity hashes for third-party scripts/styles.
func (h *ToolHandler) toolGenerateSRI(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.generateSRIImpl(req, args)
}
