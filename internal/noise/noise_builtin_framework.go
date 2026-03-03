// Purpose: Defines built-in noise rules for React, Angular, and Next.js framework warnings.
// Why: Separates framework-specific rules from analytics, browser, and devtooling categories.
package noise

func builtinFrameworkRuleSpecs() []builtinRuleSpec {
	return []builtinRuleSpec{
		{
			ID:             "builtin_react_key_warning",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `Each child in a list should have a unique.*key`,
				Level:        "warning",
			},
		},
		{
			ID:             "builtin_react_update_during_render",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `Cannot update a component.*while rendering a different component`,
				Level:        "warning",
			},
		},
		{
			ID:             "builtin_react_strict_mode",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `(StrictMode|Strict Mode).*(double|twice)`,
			},
		},
		{
			ID:             "builtin_next_hydration_info",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `(hydration|Hydration).*(mismatch|failed|warning)`,
				Level:        "warning",
			},
		},
		{
			ID:             "builtin_next_internal",
			Category:       "network",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `/_next/(static|data|image)/`,
			},
		},
		{
			ID:             "builtin_vite_client",
			Category:       "network",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `/@vite/client`,
			},
		},
		{
			ID:             "builtin_webpack_internal",
			Category:       "network",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `webpack-internal://`,
			},
		},
	}
}
