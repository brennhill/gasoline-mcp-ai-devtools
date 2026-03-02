package ai

func builtinAnalyticsRuleSpecs() []builtinRuleSpec {
	return []builtinRuleSpec{
		{
			ID:             "builtin_google_analytics",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(google-analytics\.com|analytics\.google\.com|googletagmanager\.com|gtag/js)`,
			},
		},
		{
			ID:             "builtin_segment",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(api\.segment\.(io|com)|cdn\.segment\.com)`,
			},
		},
		{
			ID:             "builtin_mixpanel",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(api\.mixpanel\.com|mxpnl\.com)`,
			},
		},
		{
			ID:             "builtin_hotjar",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `\.hotjar\.com`,
			},
		},
		{
			ID:             "builtin_amplitude",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `api\.amplitude\.com`,
			},
		},
		{
			ID:             "builtin_plausible",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `plausible\.io`,
			},
		},
		{
			ID:             "builtin_posthog",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(app\.posthog\.com|us\.posthog\.com|eu\.posthog\.com)`,
			},
		},
		{
			ID:             "builtin_datadog_rum",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `rum\.browser-intake.*\.datadoghq\.(com|eu)`,
			},
		},
		{
			ID:             "builtin_sentry",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `\.ingest\.sentry\.io`,
			},
		},
		{
			ID:             "builtin_logrocket",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(r\.lr-ingest\.io|r\.lr-in\.com)`,
			},
		},
		{
			ID:             "builtin_fullstory",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(rs\.fullstory\.com|fullstory\.com/s/fs\.js)`,
			},
		},
		{
			ID:             "builtin_heap",
			Category:       "network",
			Classification: "analytics",
			MatchSpec: NoiseMatchSpec{
				URLRegex: `(heapanalytics\.com|heap-js\.heap\.io)`,
			},
		},
	}
}
