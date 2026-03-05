// Purpose: Implements canonical-parameter alias warning and conflict response helpers.
// Why: Keeps user-facing parameter conflict diagnostics separate from runtime gate checks.

package main

import (
	"fmt"
	"strings"
)

func appendCanonicalWhatAliasWarning(resp JSONRPCResponse, aliasParam, mode string) JSONRPCResponse {
	if strings.TrimSpace(aliasParam) == "" || strings.TrimSpace(mode) == "" {
		return resp
	}
	warning := fmt.Sprintf("Accepted alias parameter '%s'; canonical parameter is 'what' (use what=%q).", aliasParam, mode)
	return appendWarningsToResponse(resp, []string{warning})
}

func whatAliasConflictResponse(req JSONRPCRequest, aliasParam, whatValue, aliasValue, validValues string) JSONRPCResponse {
	hint := "Use only 'what' when specifying tool mode/action."
	if strings.TrimSpace(validValues) != "" {
		hint += " Valid values: " + validValues
	}
	return fail(req, ErrInvalidParam,
		fmt.Sprintf("Conflicting parameters: what=%q and %s=%q", whatValue, aliasParam, aliasValue),
		"Send only the canonical 'what' parameter and retry.",
		withParam("what"), withHint(hint),
	)
}
