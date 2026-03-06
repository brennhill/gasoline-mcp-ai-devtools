// Purpose: Defines built-in noise rules for HMR WebSockets and devtools inspector connections.
// Why: Separates WebSocket noise rules from console and network noise categories.
package noise

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
