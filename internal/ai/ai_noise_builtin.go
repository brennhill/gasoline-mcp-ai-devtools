// Purpose: Owns ai_noise_builtin.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

// ai_noise_builtin.go — Built-in noise filtering rules for common browser telemetry patterns.
package ai

import "time"

// builtinRules returns the set of always-active built-in noise rules (~50 rules)
// #lizard forgives
func builtinRules() []NoiseRule {
	now := time.Now()
	return []NoiseRule{
		// ==========================================
		// Browser Internals
		// ==========================================
		{
			ID:             "builtin_chrome_extension",
			Category:       "console",
			Classification: "extension",
			MatchSpec: NoiseMatchSpec{
				SourceRegex: `(chrome|moz)-extension://`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_favicon",
			Category:       "network",
			Classification: "cosmetic",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `favicon\.ico`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_sourcemap_404",
			Category:       "network",
			Classification: "cosmetic",
			MatchSpec: NoiseMatchSpec{
				URLRegex:  `\.map(\?|$)`,
				StatusMin: 400,
				StatusMax: 499,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_cors_preflight",
			Category:       "network",
			Classification: "infrastructure",
			MatchSpec: NoiseMatchSpec{
				Method:    "OPTIONS",
				StatusMin: 200,
				StatusMax: 299,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_service_worker",
			Category:       "console",
			Classification: "infrastructure",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `(?i)(service.?worker|ServiceWorker).*(regist|install|activat|updated)`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_passive_listener",
			Category:       "console",
			Classification: "cosmetic",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `non-passive event listener`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_deprecation",
			Category:       "console",
			Classification: "cosmetic",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `^\[Deprecation\]`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_devtools_sourcemap",
			Category:       "console",
			Classification: "cosmetic",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `DevTools failed to load source map`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_err_blocked",
			Category:       "console",
			Classification: "extension",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `net::ERR_BLOCKED_BY_CLIENT`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_samesite_cookie",
			Category:       "console",
			Classification: "cosmetic",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `Indicate whether to send a cookie`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_third_party_cookie",
			Category:       "console",
			Classification: "cosmetic",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `third-party cookie will be blocked`,
			},
			CreatedAt: now,
		},

		// ==========================================
		// Dev Tooling
		// ==========================================
		{
			ID:             "builtin_hmr_console",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `^\[(vite|HMR|webpack|next)\]`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_hmr_network",
			Category:       "network",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(__vite_ping|hot-update\.(json|js)|__webpack_hmr|sockjs-node|_next/webpack-hmr|webpack-dev-server)`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_react_devtools",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `(Download the React DevTools|React DevTools)`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_angular_dev_mode",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `Angular is running in (the )?development mode`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_vue_devtools",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `(Vue\.js|vue-devtools|Vue Devtools)`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_svelte_hmr",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `\[svelte-hmr\]`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_fast_refresh",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `\[Fast Refresh\]`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_next_dev",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `next-dev\.js`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_vite_prebundle",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `(Pre-bundling|Optimized dependencies|new dependencies optimized)`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_cra_disconnect",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `The development server has disconnected`,
			},
			CreatedAt: now,
		},

		// ==========================================
		// Analytics & Tracking
		// ==========================================
		{
			ID:             "builtin_google_analytics",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(google-analytics\.com|analytics\.google\.com|googletagmanager\.com|gtag/js)`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_segment",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(api\.segment\.(io|com)|cdn\.segment\.com)`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_mixpanel",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(api\.mixpanel\.com|mxpnl\.com)`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_hotjar",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `\.hotjar\.com`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_amplitude",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `api\.amplitude\.com`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_plausible",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `plausible\.io`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_posthog",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(app\.posthog\.com|us\.posthog\.com|eu\.posthog\.com)`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_datadog_rum",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `rum\.browser-intake.*\.datadoghq\.(com|eu)`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_sentry",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `\.ingest\.sentry\.io`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_logrocket",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(r\.lr-ingest\.io|r\.lr-in\.com)`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_fullstory",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(rs\.fullstory\.com|fullstory\.com/s/fs\.js)`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_heap",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(heapanalytics\.com|heap-js\.heap\.io)`,
			},
			CreatedAt: now,
		},

		// ==========================================
		// Framework Noise (common patterns)
		// ==========================================
		{
			ID:             "builtin_react_key_warning",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `Each child in a list should have a unique.*key`,
				Level:        "warning",
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_react_update_during_render",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `Cannot update a component.*while rendering a different component`,
				Level:        "warning",
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_react_strict_mode",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `(StrictMode|Strict Mode).*(double|twice)`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_next_hydration_info",
			Category:       "console",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `(hydration|Hydration).*(mismatch|failed|warning)`,
				Level:        "warning",
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_next_internal",
			Category:       "network",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `/_next/(static|data|image)/`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_vite_client",
			Category:       "network",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `/@vite/client`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_webpack_internal",
			Category:       "network",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `webpack-internal://`,
			},
			CreatedAt: now,
		},

		// ==========================================
		// WebSocket Noise
		// ==========================================
		{
			ID:             "builtin_ws_hmr",
			Category:       "websocket",
			Classification: "framework",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(/__vite_hmr|localhost(:\d+)?/ws(\?|$)|/_next/webpack-hmr|/sockjs-node)`,
			},
			CreatedAt: now,
		},
		{
			ID:             "builtin_ws_devtools",
			Category:       "websocket",
			Classification: "extension",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(devtools|__browser_inspector)`,
			},
			CreatedAt: now,
		},
	}
}
