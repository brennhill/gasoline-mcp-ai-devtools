// Purpose: Defines built-in baseline noise rules shipped with AI noise filtering.
// Why: Provides sensible default suppression coverage before user-defined rules are added.
// Docs: docs/features/feature/noise-filtering/index.md

package noise

import "time"

type builtinRuleSpec struct {
	ID             string
	Category       string
	Classification string
	MatchSpec      NoiseMatchSpec
}

func builtinRules() []NoiseRule {
	now := time.Now()
	specs := collectBuiltinRuleSpecs()
	rules := make([]NoiseRule, 0, len(specs))
	for _, spec := range specs {
		rules = append(rules, NoiseRule{
			ID:             spec.ID,
			Category:       spec.Category,
			Classification: spec.Classification,
			MatchSpec:      spec.MatchSpec,
			CreatedAt:      now,
		})
	}
	return rules
}

func collectBuiltinRuleSpecs() []builtinRuleSpec {
	specs := make([]builtinRuleSpec, 0, 64)
	specs = append(specs, builtinBrowserRuleSpecs()...)
	specs = append(specs, builtinDevToolingRuleSpecs()...)
	specs = append(specs, builtinAnalyticsRuleSpecs()...)
	specs = append(specs, builtinFrameworkRuleSpecs()...)
	specs = append(specs, builtinWebSocketRuleSpecs()...)
	return specs
}
