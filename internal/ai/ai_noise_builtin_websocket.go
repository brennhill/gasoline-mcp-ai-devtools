package ai

func builtinWebSocketRuleSpecs() []builtinRuleSpec {
	return []builtinRuleSpec{
		{
			ID:             "builtin_ws_hmr",
			Category:       "websocket",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(/__vite_hmr|localhost(:\d+)?/ws(\?|$)|/_next/webpack-hmr|/sockjs-node)`,
			},
		},
		{
			ID:             "builtin_ws_devtools",
			Category:       "websocket",
			Classification: "extension",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(devtools|__browser_inspector)`,
			},
		},
	}
}
