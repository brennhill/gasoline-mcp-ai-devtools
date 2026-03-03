// mode_specs_interact.go — interact tool per-mode parameter specs.
// Docs: docs/features/describe_capabilities.md
package configure

import "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/schema"

// interactModeSpecs derives directly from the canonical interact action registry
// in internal/schema/interact_actions.go to keep schema + capabilities in sync.
var interactModeSpecs = buildInteractModeSpecs()

func buildInteractModeSpecs() map[string]modeParamSpec {
	specs := schema.InteractActionSpecs()
	out := make(map[string]modeParamSpec, len(specs))
	for _, spec := range specs {
		out[spec.Name] = modeParamSpec{
			Hint:     spec.Hint,
			Required: append([]string(nil), spec.Required...),
			Optional: append([]string(nil), spec.Optional...),
		}
	}
	return out
}
