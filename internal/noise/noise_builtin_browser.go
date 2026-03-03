package noise

func builtinBrowserRuleSpecs() []builtinRuleSpec {
	return []builtinRuleSpec{
		{
			ID:             "builtin_chrome_extension",
			Category:       "console",
			Classification: "extension",
			MatchSpec: NoiseMatchSpec{
				SourceRegex: `(chrome|moz)-extension://`,
			},
		},
		{
			ID:             "builtin_favicon",
			Category:       "network",
			Classification: "cosmetic",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `favicon\.ico`,
			},
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
		},
		{
			ID:             "builtin_service_worker",
			Category:       "console",
			Classification: "infrastructure",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `(?i)(service.?worker|ServiceWorker).*(regist|install|activat|updated)`,
			},
		},
		{
			ID:             "builtin_passive_listener",
			Category:       "console",
			Classification: "cosmetic",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `non-passive event listener`,
			},
		},
		{
			ID:             "builtin_deprecation",
			Category:       "console",
			Classification: "cosmetic",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `^\[Deprecation\]`,
			},
		},
		{
			ID:             "builtin_devtools_sourcemap",
			Category:       "console",
			Classification: "cosmetic",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `DevTools failed to load source map`,
			},
		},
		{
			ID:             "builtin_err_blocked",
			Category:       "console",
			Classification: "extension",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `net::ERR_BLOCKED_BY_CLIENT`,
			},
		},
		{
			ID:             "builtin_samesite_cookie",
			Category:       "console",
			Classification: "cosmetic",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `Indicate whether to send a cookie`,
			},
		},
		{
			ID:             "builtin_third_party_cookie",
			Category:       "console",
			Classification: "cosmetic",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: `third-party cookie will be blocked`,
			},
		},
	}
}
