// Purpose: Defines structured recovery playbooks for interact action failures (element_not_found, ambiguous_target, stale handle, etc.).
// Why: Enables deterministic agent self-recovery by embedding ordered retry steps and stop conditions in error responses.
// Docs: docs/features/feature/interact-explore/index.md

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
			`Pick the correct element from the candidates array in this response (use suggested_element_id if present).`,
			`Retry the same action with element_id instead of the ambiguous selector.`,
			`If none of the candidates match your intent, use scope_selector to narrow the search area.`,
		},
		StopAndReportCondition: "If ambiguity persists after one scoped retry, stop and report candidate list evidence instead of repeated blind clicks.",
		RetrySuggestion:        `Recovery: pick from the candidates array below and retry with element_id (or use suggested_element_id). No extra list_interactive call needed.`,
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
	"blocked_by_overlay": {
		DetectionSignal: "error=blocked_by_overlay (element obscured by a modal/dialog/overlay)",
		OrderedRecoverySteps: []string{
			`Run interact({what:"dismiss_top_overlay"}) to close the topmost modal/dialog.`,
			`If dismiss_top_overlay fails, try interact({what:"key_press", text:"Escape"}) as a fallback.`,
			`Retry the original action after the overlay is dismissed.`,
		},
		StopAndReportCondition: "If dismiss_top_overlay and Escape both fail, take a screenshot and report the overlay. The page may require manual intervention.",
		RetrySuggestion:        `Recovery: run interact({what:"dismiss_top_overlay"}) first, then retry the original action.`,
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
