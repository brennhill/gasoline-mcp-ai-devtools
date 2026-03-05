// Purpose: Generate-dispatch parameter validation and warning filtering helpers.
// Why: Keeps generate router logic focused on format dispatch.

package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// generateValidParams defines the allowed parameter names per generate format.
// The "format" and "telemetry_mode" params are always allowed.
var generateValidParams = map[string]map[string]bool{
	"reproduction":      {"error_message": true, "last_n": true, "base_url": true, "include_screenshots": true, "generate_fixtures": true, "visual_assertions": true, "save_to": true, "output_format": true},
	"test":              {"test_name": true, "last_n": true, "base_url": true, "assert_network": true, "assert_no_errors": true, "assert_response_shape": true, "save_to": true},
	"pr_summary":        {"save_to": true},
	"har":               {"url": true, "method": true, "status_min": true, "status_max": true, "save_to": true},
	"csp":               {"mode": true, "include_report_uri": true, "exclude_origins": true, "save_to": true},
	"sri":               {"resource_types": true, "origins": true, "save_to": true},
	"sarif":             {"scope": true, "include_passes": true, "save_to": true},
	"visual_test":       {"test_name": true, "annot_session": true, "save_to": true},
	"annotation_report": {"annot_session": true, "save_to": true},
	"annotation_issues": {"annot_session": true, "save_to": true},
	"test_from_context": {"context": true, "error_id": true, "include_mocks": true, "output_format": true, "save_to": true},
	"test_heal":         {"action": true, "test_file": true, "test_dir": true, "broken_selectors": true, "auto_apply": true, "save_to": true},
	"test_classify":     {"action": true, "failure": true, "failures": true, "save_to": true},
}

// alwaysAllowedGenerateParams are params valid for every generate format.
var alwaysAllowedGenerateParams = map[string]bool{
	"what":           true,
	"format":         true,
	"telemetry_mode": true,
}

// ignoredGenerateDispatchWarningParams are accepted at generate-dispatch level
// but not consumed by every sub-handler.
var ignoredGenerateDispatchWarningParams = map[string]bool{
	"what":           true,
	"format":         true,
	"telemetry_mode": true,
	"save_to":        true,
}

func filterGenerateDispatchWarnings(warnings []string) []string {
	if len(warnings) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		param, ok := parseUnknownParamWarning(warning)
		if ok && ignoredGenerateDispatchWarningParams[param] {
			continue
		}
		filtered = append(filtered, warning)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func parseUnknownParamWarning(warning string) (string, bool) {
	const prefix = "unknown parameter '"
	const suffix = "' (ignored)"
	if !strings.HasPrefix(warning, prefix) || !strings.HasSuffix(warning, suffix) {
		return "", false
	}
	param := strings.TrimPrefix(warning, prefix)
	param = strings.TrimSuffix(param, suffix)
	if param == "" {
		return "", false
	}
	return param, true
}

// validateGenerateParams checks for unknown parameters and returns an error response if any are found.
func validateGenerateParams(req JSONRPCRequest, format string, args json.RawMessage) *JSONRPCResponse {
	if len(args) == 0 {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(args, &raw); err != nil {
		return nil // let handler deal with bad JSON
	}
	allowed, ok := generateValidParams[format]
	if !ok {
		return nil // unknown format handled elsewhere
	}
	var unknown []string
	for k := range raw {
		if alwaysAllowedGenerateParams[k] || allowed[k] {
			continue
		}
		unknown = append(unknown, k)
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	validList := make([]string, 0, len(allowed))
	for k := range allowed {
		validList = append(validList, k)
	}
	sort.Strings(validList)
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
		ErrInvalidParam,
		fmt.Sprintf("Unknown parameter(s) for format '%s': %s", format, strings.Join(unknown, ", ")),
		"Remove unknown parameters and call again",
		withParam(unknown[0]),
		withHint(fmt.Sprintf("Valid params for '%s': %s", format, strings.Join(validList, ", "))),
	)}
	return &resp
}
