// mode_specs.go — Per-mode parameter specs for all tools.
// Docs: docs/features/describe_capabilities.md
package configure

// toolModeSpecs maps tool name → mode name → { Hint, Required, Optional }.
// Each mode lists only the params relevant to that mode, preventing
// the full param list from being dumped into every mode's output.
// Hint is a one-line description surfaced in summary mode for discovery.
var toolModeSpecs = map[string]map[string]modeParamSpec{
	"configure": configureModeSpecs,
	"observe":   observeModeSpecs,
	"interact":  interactModeSpecs,
	"analyze":   analyzeModeSpecs,
	"generate":  generateModeSpecs,
}
