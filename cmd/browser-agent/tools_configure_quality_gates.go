// tools_configure_quality_gates.go — Thin dispatch for configure(what="setup_quality_gates").
// Why: Business logic lives in internal/toolconfigure; this file handles MCP plumbing only.
// Docs: docs/features/feature/quality-gates/index.md

package main

import (
	"encoding/json"
	"errors"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure"
)

// toolConfigureSetupQualityGates handles configure(what="setup_quality_gates").
func (h *ToolHandler) toolConfigureSetupQualityGates(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TargetDir string `json:"target_dir"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	projectDir := h.server.GetActiveCodebase()
	if projectDir == "" {
		return fail(req, ErrNotInitialized,
			"No active codebase set. Cannot determine project directory.",
			"Set active_codebase via configure(what='store', key='active_codebase', data='<path>') first")
	}

	result, err := toolconfigure.SetupQualityGates(projectDir, params.TargetDir)
	if err != nil {
		var pathErr *toolconfigure.PathNotAllowedError
		if errors.As(err, &pathErr) {
			return fail(req, ErrPathNotAllowed,
				pathErr.Error(),
				"Use a path within "+pathErr.Project, withParam("target_dir"))
		}
		var dirErr *toolconfigure.TargetNotDirError
		if errors.As(err, &dirErr) {
			return fail(req, ErrInvalidParam, dirErr.Error(),
				"Provide an existing directory path", withParam("target_dir"))
		}
		return fail(req, ErrInternal, err.Error(), "Check file system permissions")
	}

	return succeed(req, result.Summary, result.Data)
}
