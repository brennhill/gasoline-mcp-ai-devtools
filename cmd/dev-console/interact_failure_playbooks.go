package main

import "strings"

type interactFailurePlaybook struct {
	DetectionSignal        string
	OrderedRecoverySteps   []string
	StopAndReportCondition string
	RetrySuggestion        string
}

var interactFailurePlaybooks = map[string]interactFailurePlaybook{
	"element_not_found": {
		DetectionSignal: "error=element_not_found",
		OrderedRecoverySteps: []string{
			`Run interact({what:"list_interactive", scope_selector:"<container>"}) to refresh candidates.`,
			`Retry action using element_id/index from scoped list_interactive results instead of a broad selector.`,
			`If still empty, widen or remove scope_selector once and retry.`,
		},
		StopAndReportCondition: "If one scoped refresh and one scope-widening retry both fail, stop and report evidence (command_result + screenshot + scoped list_interactive output).",
		RetrySuggestion:        `Recovery: run list_interactive in the intended scope, retry with element_id/index, then widen scope_selector once if needed.`,
	},
	"ambiguous_target": {
		DetectionSignal: "error=ambiguous_target",
		OrderedRecoverySteps: []string{
			`Add scope_selector or scope_rect to narrow candidates to the active container.`,
			`Use list_interactive in that scope and choose element_id/index instead of global text selector.`,
			`Retry the action with the resolved element_id/index.`,
		},
		StopAndReportCondition: "If ambiguity persists after one scoped retry, stop and report candidate list evidence instead of repeated blind clicks.",
		RetrySuggestion:        `Recovery: add scope_selector/scope_rect, run list_interactive, then retry using element_id/index.`,
	},
	"stale_element_id": {
		DetectionSignal: "error=stale_element_id",
		OrderedRecoverySteps: []string{
			`Refresh handles with interact({what:"list_interactive", ...same scope...}).`,
			`Reacquire a new element_id (or index) for the same target label/role.`,
			`Retry the action with the refreshed element_id.`,
		},
		StopAndReportCondition: "If refreshed handles immediately go stale again, stop and report evidence (likely DOM churn/rerender race).",
		RetrySuggestion:        `Recovery: refresh list_interactive, reacquire element_id, and retry once with the new handle.`,
	},
	"scope_not_found": {
		DetectionSignal: "error=scope_not_found",
		OrderedRecoverySteps: []string{
			`Try a fallback scope_selector that matches an active dialog/container on the current page.`,
			`If selector scoping fails, use scope_rect (annotation region) or frame targeting when content is embedded.`,
			`Re-run list_interactive in the recovered scope before mutating actions.`,
		},
		StopAndReportCondition: "If selector scope, scope_rect, and frame fallback all fail, stop and report evidence (page screenshot + available frames/selectors).",
		RetrySuggestion:        `Recovery: adjust scope_selector, then try scope_rect/frame fallback, then rerun list_interactive before retrying action.`,
	},
}

func tutorialFailureRecoveryPlaybooks() map[string]any {
	out := make(map[string]any, len(interactFailurePlaybooks))
	for code, playbook := range interactFailurePlaybooks {
		out[code] = map[string]any{
			"detection_signal":          playbook.DetectionSignal,
			"ordered_recovery_steps":    playbook.OrderedRecoverySteps,
			"stop_and_report_condition": playbook.StopAndReportCondition,
			"retry_guidance":            playbook.RetrySuggestion,
		}
	}
	return out
}

func lookupInteractFailurePlaybook(rawCode string) (string, interactFailurePlaybook, bool) {
	code := normalizeInteractFailureCode(rawCode)
	if code == "" {
		return "", interactFailurePlaybook{}, false
	}
	playbook, ok := interactFailurePlaybooks[code]
	if !ok {
		return "", interactFailurePlaybook{}, false
	}
	return code, playbook, true
}

func normalizeInteractFailureCode(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return ""
	}
	for code := range interactFailurePlaybooks {
		if v == code || strings.Contains(v, code) {
			return code
		}
	}
	return ""
}
