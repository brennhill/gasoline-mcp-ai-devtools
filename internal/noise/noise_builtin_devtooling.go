package noise

func builtinDevToolingRuleSpecs() []builtinRuleSpec {
	return []builtinRuleSpec{
		{
			ID:             "builtin_hmr_console",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `^\[(vite|HMR|webpack|next)\]`,
			},
		},
		{
			ID:             "builtin_hmr_network",
			Category:       "network",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(__vite_ping|hot-update\.(json|js)|__webpack_hmr|sockjs-node|_next/webpack-hmr|webpack-dev-server)`,
			},
		},
		{
			ID:             "builtin_react_devtools",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `(Download the React DevTools|React DevTools)`,
			},
		},
		{
			ID:             "builtin_angular_dev_mode",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `Angular is running in (the )?development mode`,
			},
		},
		{
			ID:             "builtin_vue_devtools",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `(Vue\.js|vue-devtools|Vue Devtools)`,
			},
		},
		{
			ID:             "builtin_svelte_hmr",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `\[svelte-hmr\]`,
			},
		},
		{
			ID:             "builtin_fast_refresh",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `\[Fast Refresh\]`,
			},
		},
		{
			ID:             "builtin_next_dev",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `next-dev\.js`,
			},
		},
		{
			ID:             "builtin_vite_prebundle",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `(Pre-bundling|Optimized dependencies|new dependencies optimized)`,
			},
		},
		{
			ID:             "builtin_cra_disconnect",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `The development server has disconnected`,
			},
		},
	}
}
