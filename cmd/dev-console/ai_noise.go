// ai_noise.go — Noise filtering rules and auto-detection for browser telemetry.
// Provides regex-based match specs that classify console, network, and WebSocket
// entries as noise (third-party scripts, analytics, browser-generated warnings).
// Auto-detection analyzes buffer contents to suggest rules for high-frequency patterns.
// Design: Rules are AND-matched (all fields must match), stored in a mutex-guarded
// slice with a hard cap of 100 rules. Dismissed patterns use a separate quick-match list.
package main

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================
// Noise Filtering
// ============================================

const maxNoiseRules = 100

// NoiseMatchSpec defines how a rule matches entries
type NoiseMatchSpec struct {
	MessageRegex string `json:"message_regex,omitempty"`
	SourceRegex  string `json:"source_regex,omitempty"` 
	URLRegex     string `json:"url_regex,omitempty"`    
	Method       string `json:"method,omitempty"`
	StatusMin    int    `json:"status_min,omitempty"`
	StatusMax    int    `json:"status_max,omitempty"`
	Level        string `json:"level,omitempty"`
}

// NoiseRule represents a single noise filtering rule
type NoiseRule struct {
	ID             string         `json:"id"`
	Category       string         `json:"category"`       // "console", "network", "websocket"
	Classification string         `json:"classification"` // "extension", "framework", "cosmetic", "analytics", "infrastructure", "repetitive", "dismissed"
	MatchSpec      NoiseMatchSpec `json:"match_spec"`     
	AutoDetected   bool           `json:"auto_detected,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`     
	Reason         string         `json:"reason,omitempty"`
}

// compiledRule holds a rule with pre-compiled regex patterns
type compiledRule struct {
	rule         NoiseRule
	messageRegex *regexp.Regexp
	sourceRegex  *regexp.Regexp
	urlRegex     *regexp.Regexp
}

// NoiseProposal represents an auto-detected noise proposal
type NoiseProposal struct {
	Rule       NoiseRule `json:"rule"`
	Confidence float64   `json:"confidence"`
	Reason     string    `json:"reason"`
}

// NoiseStatistics tracks filtering metrics
type NoiseStatistics struct {
	TotalFiltered int64          `json:"total_filtered"`
	PerRule       map[string]int `json:"per_rule"`      
	LastSignalAt  time.Time      `json:"last_signal_at,omitempty"` 
	LastNoiseAt   time.Time      `json:"last_noise_at,omitempty"`  
}

// NoiseConfig manages noise filtering rules and state
// NoiseConfig manages noise filtering rules with dual-mutex concurrency control.
//
// LOCK ORDERING INVARIANT (H-5 documented):
//   mu -> statsMu (always acquire mu before statsMu, never the reverse)
//
// This ordering is enforced throughout the codebase:
//   - Filter methods (IsNoise, IsNetworkNoise, IsWSNoise) acquire mu.RLock first,
//     then call recordMatch/recordSignal which acquire statsMu.
//   - GetStatistics acquires only statsMu (no mu held).
//   - AutoDetect acquires only mu (no statsMu held).
//
// If future code needs both locks simultaneously, it MUST acquire mu first.
// Violating this ordering will cause deadlock.
type NoiseConfig struct {
	mu            sync.RWMutex
	rules         []NoiseRule
	compiled      []compiledRule
	statsMu       sync.Mutex // separate mutex for stats (written during reads)
	stats         NoiseStatistics
	userIDCounter int
}

// NewNoiseConfig creates a new NoiseConfig with built-in rules
func NewNoiseConfig() *NoiseConfig {
	nc := &NoiseConfig{
		stats: NoiseStatistics{
			PerRule: make(map[string]int),
		},
	}

	nc.rules = builtinRules()
	nc.recompile()
	return nc
}

// builtinRules returns the set of always-active built-in noise rules (~50 rules)
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

// recompile compiles all regex patterns in rules. Invalid regexes result in nil (never match).
func (nc *NoiseConfig) recompile() {
	compiled := make([]compiledRule, len(nc.rules))
	for i := range nc.rules {
		r := &nc.rules[i]
		cr := compiledRule{rule: *r}
		if r.MatchSpec.MessageRegex != "" {
			re, err := regexp.Compile(r.MatchSpec.MessageRegex)
			if err == nil {
				cr.messageRegex = re
			}
		}
		if r.MatchSpec.SourceRegex != "" {
			re, err := regexp.Compile(r.MatchSpec.SourceRegex)
			if err == nil {
				cr.sourceRegex = re
			}
		}
		if r.MatchSpec.URLRegex != "" {
			re, err := regexp.Compile(r.MatchSpec.URLRegex)
			if err == nil {
				cr.urlRegex = re
			}
		}
		compiled[i] = cr
	}
	nc.compiled = compiled
}

// ListRules returns a copy of all current rules
func (nc *NoiseConfig) ListRules() []NoiseRule {
	nc.mu.RLock()
	defer nc.mu.RUnlock()
	result := make([]NoiseRule, len(nc.rules))
	copy(result, nc.rules)
	return result
}

// validateRegexPattern checks if a regex pattern is safe to compile.
// Rejects patterns with excessive length or nested quantifiers that could
// cause significant performance degradation.
// Returns nil if the pattern is safe (even if it has invalid syntax - those are caught during compilation).
func validateRegexPattern(pattern string) error {
	const maxPatternLength = 512

	if len(pattern) > maxPatternLength {
		return fmt.Errorf("regex pattern exceeds maximum length of %d characters", maxPatternLength)
	}

	// Detect nested quantifiers: quantifier followed by another quantifier
	// This pattern checks for: quantifier (+, *, ?, {n,m}) followed by optional whitespace/group close, then another quantifier
	// Examples: (a+)+, (b*)+, (c?)+, (d{1,2})+, etc.
	nestedQuantifierPatterns := []string{
		`\+\s*\)?\s*[\+\*\?]`,    // + followed by optional ) and another quantifier
		`\*\s*\)?\s*[\+\*\?]`,    // * followed by optional ) and another quantifier
		`\?\s*\)?\s*[\+\*\?]`,    // ? followed by optional ) and another quantifier
		`\}\s*\)?\s*[\+\*\?]`,    // {n,m} followed by optional ) and another quantifier
	}

	for _, np := range nestedQuantifierPatterns {
		if matched, _ := regexp.MatchString(np, pattern); matched {
			return fmt.Errorf("regex pattern contains nested quantifiers which can cause performance issues")
		}
	}

	// NOTE: We do NOT reject patterns with invalid syntax here.
	// Invalid syntax will fail during regexp.Compile() in recompile(),
	// and those rules will be silently skipped (compiled field remains nil).
	// This maintains backward compatibility with existing behavior.

	return nil
}

// AddRules adds user rules to the config. Rules exceeding max are silently dropped.
func (nc *NoiseConfig) AddRules(rules []NoiseRule) error {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	// Validate all rules before adding any
	for i := range rules {
		if rules[i].MatchSpec.MessageRegex != "" {
			if err := validateRegexPattern(rules[i].MatchSpec.MessageRegex); err != nil {
				return fmt.Errorf("invalid MessageRegex in rule: %w", err)
			}
		}
		if rules[i].MatchSpec.SourceRegex != "" {
			if err := validateRegexPattern(rules[i].MatchSpec.SourceRegex); err != nil {
				return fmt.Errorf("invalid SourceRegex in rule: %w", err)
			}
		}
		if rules[i].MatchSpec.URLRegex != "" {
			if err := validateRegexPattern(rules[i].MatchSpec.URLRegex); err != nil {
				return fmt.Errorf("invalid URLRegex in rule: %w", err)
			}
		}
	}

	for i := range rules {
		if len(nc.rules) >= maxNoiseRules {
			break
		}
		nc.userIDCounter++
		rules[i].ID = fmt.Sprintf("user_%d", nc.userIDCounter)
		rules[i].CreatedAt = time.Now()
		nc.rules = append(nc.rules, rules[i])
	}
	nc.recompile()
	return nil
}

// RemoveRule removes a rule by ID. Built-in rules cannot be removed.
func (nc *NoiseConfig) RemoveRule(id string) error {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	if strings.HasPrefix(id, "builtin_") {
		return fmt.Errorf("cannot remove built-in rule: %s", id)
	}

	for i := range nc.rules {
		if nc.rules[i].ID == id {
			nc.rules = append(nc.rules[:i], nc.rules[i+1:]...)
			nc.recompile()
			return nil
		}
	}
	return fmt.Errorf("rule not found: %s", id)
}

// Reset removes all user/auto rules, reverting to only built-ins.
func (nc *NoiseConfig) Reset() {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	nc.rules = builtinRules()
	nc.recompile()
	nc.stats = NoiseStatistics{
		PerRule: make(map[string]int),
	}
}

// IsConsoleNoise checks if a console log entry matches any noise rule.
func (nc *NoiseConfig) IsConsoleNoise(entry LogEntry) bool {
	nc.mu.RLock()
	defer nc.mu.RUnlock()

	message, _ := entry["message"].(string)
	source, _ := entry["source"].(string)
	level, _ := entry["level"].(string)

	for i := range nc.compiled {
		cr := &nc.compiled[i]
		if cr.rule.Category != "console" {
			continue
		}

		// Check level filter
		if cr.rule.MatchSpec.Level != "" && cr.rule.MatchSpec.Level != level {
			continue
		}

		// Either message regex or source regex must match (OR logic)
		matched := false
		if cr.messageRegex != nil && cr.messageRegex.MatchString(message) {
			matched = true
		}
		if cr.sourceRegex != nil && cr.sourceRegex.MatchString(source) {
			matched = true
		}

		if matched {
			nc.recordMatch(cr.rule.ID)
			return true
		}
	}

	nc.recordSignal()
	return false
}

// IsNetworkNoise checks if a network body matches any noise rule.
// Security invariant: 401/403 responses are NEVER noise.
func (nc *NoiseConfig) IsNetworkNoise(body NetworkBody) bool {
	// Security invariant: auth responses never filtered
	if body.Status == 401 || body.Status == 403 {
		return false
	}

	nc.mu.RLock()
	defer nc.mu.RUnlock()

	for i := range nc.compiled {
		cr := &nc.compiled[i]
		if cr.rule.Category != "network" {
			continue
		}

		// Check method filter
		if cr.rule.MatchSpec.Method != "" && cr.rule.MatchSpec.Method != body.Method {
			continue
		}

		// Check status range
		if cr.rule.MatchSpec.StatusMin > 0 && body.Status < cr.rule.MatchSpec.StatusMin {
			continue
		}
		if cr.rule.MatchSpec.StatusMax > 0 && body.Status > cr.rule.MatchSpec.StatusMax {
			continue
		}

		// Check URL regex
		if cr.urlRegex != nil && cr.urlRegex.MatchString(body.URL) {
			nc.recordMatch(cr.rule.ID)
			return true
		}

		// If no URL regex set but method/status matched, it's a match
		// (e.g., OPTIONS preflight rule with only method and status range)
		if cr.rule.MatchSpec.URLRegex == "" && (cr.rule.MatchSpec.Method != "" || cr.rule.MatchSpec.StatusMin > 0) {
			nc.recordMatch(cr.rule.ID)
			return true
		}
	}

	nc.recordSignal()
	return false
}

// IsWebSocketNoise checks if a WebSocket event matches any noise rule.
func (nc *NoiseConfig) IsWebSocketNoise(event WebSocketEvent) bool {
	nc.mu.RLock()
	defer nc.mu.RUnlock()

	for i := range nc.compiled {
		cr := &nc.compiled[i]
		if cr.rule.Category != "websocket" {
			continue
		}

		if cr.urlRegex != nil && cr.urlRegex.MatchString(event.URL) {
			nc.recordMatch(cr.rule.ID)
			return true
		}
	}

	nc.recordSignal()
	return false
}

// recordMatch updates statistics for a matched rule (thread-safe via statsMu)
func (nc *NoiseConfig) recordMatch(ruleID string) {
	nc.statsMu.Lock()
	nc.stats.TotalFiltered++
	nc.stats.PerRule[ruleID]++
	nc.stats.LastNoiseAt = time.Now()
	nc.statsMu.Unlock()
}

// recordSignal updates the last signal timestamp (thread-safe via statsMu)
func (nc *NoiseConfig) recordSignal() {
	nc.statsMu.Lock()
	nc.stats.LastSignalAt = time.Now()
	nc.statsMu.Unlock()
}

// GetStatistics returns a copy of the current noise statistics
func (nc *NoiseConfig) GetStatistics() NoiseStatistics {
	nc.statsMu.Lock()
	defer nc.statsMu.Unlock()
	perRule := make(map[string]int, len(nc.stats.PerRule))
	for k, v := range nc.stats.PerRule {
		perRule[k] = v
	}
	return NoiseStatistics{
		TotalFiltered: nc.stats.TotalFiltered,
		PerRule:       perRule,
		LastSignalAt:  nc.stats.LastSignalAt,
		LastNoiseAt:   nc.stats.LastNoiseAt,
	}
}

// DismissNoise is a convenience method that creates a "dismissed" rule from a pattern.
// If category is empty, defaults to "console".
func (nc *NoiseConfig) DismissNoise(pattern string, category string, reason string) {
	if category == "" {
		category = "console"
	}

	nc.mu.Lock()
	defer nc.mu.Unlock()

	if len(nc.rules) >= maxNoiseRules {
		return
	}

	nc.userIDCounter++
	rule := NoiseRule{
		ID:             fmt.Sprintf("dismiss_%d", nc.userIDCounter),
		Category:       category,
		Classification: "dismissed",
		CreatedAt:      time.Now(),
		Reason:         reason,
	}

	switch category {
	case "network":
		rule.MatchSpec.URLRegex = pattern
	case "websocket":
		rule.MatchSpec.URLRegex = pattern
	default: // console
		rule.MatchSpec.MessageRegex = pattern
	}

	nc.rules = append(nc.rules, rule)
	nc.recompile()
}

// AutoDetect analyzes buffers and proposes noise rules based on frequency and source analysis.
// High-confidence proposals (>= 0.9) are automatically applied.
// Note: This function holds a write lock for the entire analysis. It is designed for
// infrequent manual invocation via the MCP tool, not for hot-path usage.
func (nc *NoiseConfig) AutoDetect(consoleEntries []LogEntry, networkBodies []NetworkBody, wsEvents []WebSocketEvent) []NoiseProposal {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	proposals := make([]NoiseProposal, 0)

	// --- Frequency analysis for console messages ---
	if len(consoleEntries) > 0 {
		msgCounts := make(map[string]int)
		for _, entry := range consoleEntries {
			msg, _ := entry["message"].(string)
			if msg != "" {
				msgCounts[msg]++
			}
		}

		for msg, count := range msgCounts {
			if count < 10 {
				continue
			}

			// Check if already covered by existing rules
			if nc.isConsoleCoveredLocked(msg, consoleEntries) {
				continue
			}

			confidence := 0.7 + float64(count)/100.0
			if confidence > 0.99 {
				confidence = 0.99
			}

			proposal := NoiseProposal{
				Rule: NoiseRule{
					Category:       "console",
					Classification: "repetitive",
					AutoDetected:   true,
					MatchSpec: NoiseMatchSpec{
						MessageRegex: regexp.QuoteMeta(msg),
					},
				},
				Confidence: confidence,
				Reason:     fmt.Sprintf("message repeated %d times", count),
			}
			proposals = append(proposals, proposal)
		}
	}

	// --- Source analysis for console entries ---
	if len(consoleEntries) > 0 {
		sourceCounts := make(map[string][]LogEntry)
		for _, entry := range consoleEntries {
			source, _ := entry["source"].(string)
			if source != "" && strings.Contains(source, "node_modules") {
				// Group by source path
				sourceCounts[source] = append(sourceCounts[source], entry)
			}
		}

		for source, entries := range sourceCounts {
			if len(entries) < 2 {
				continue
			}
			// Check if already covered
			if nc.isSourceCoveredLocked(source) {
				continue
			}

			proposal := NoiseProposal{
				Rule: NoiseRule{
					Category:       "console",
					Classification: "extension",
					AutoDetected:   true,
					MatchSpec: NoiseMatchSpec{
						SourceRegex: regexp.QuoteMeta(source),
					},
				},
				Confidence: 0.75,
				Reason:     fmt.Sprintf("node_modules source with %d entries", len(entries)),
			}
			proposals = append(proposals, proposal)
		}
	}

	// --- Network frequency analysis ---
	if len(networkBodies) > 0 {
		urlCounts := make(map[string]int)
		for _, body := range networkBodies {
			// Extract path from URL
			path := extractURLPath(body.URL)
			if path != "" {
				urlCounts[path]++
			}
		}

		infraPatterns := []string{"/health", "/ping", "/ready", "/__", "/sockjs-node", "/ws"}
		for path, count := range urlCounts {
			if count < 20 {
				continue
			}
			// Check if it looks like infrastructure
			isInfra := false
			for _, pat := range infraPatterns {
				if strings.Contains(path, pat) {
					isInfra = true
					break
				}
			}
			if !isInfra {
				continue
			}

			// Check if already covered
			if nc.isURLCoveredLocked(path) {
				continue
			}

			proposal := NoiseProposal{
				Rule: NoiseRule{
					Category:       "network",
					Classification: "infrastructure",
					AutoDetected:   true,
					MatchSpec: NoiseMatchSpec{
						URLRegex: regexp.QuoteMeta(path),
					},
				},
				Confidence: 0.8,
				Reason:     fmt.Sprintf("infrastructure path hit %d times", count),
			}
			proposals = append(proposals, proposal)
		}
	}

	// Auto-apply high-confidence proposals
	for i := range proposals {
		if proposals[i].Confidence < 0.9 || len(nc.rules) >= maxNoiseRules {
			continue
		}
		nc.userIDCounter++
		rule := proposals[i].Rule
		rule.ID = fmt.Sprintf("auto_%d", nc.userIDCounter)
		rule.CreatedAt = time.Now()
		nc.rules = append(nc.rules, rule)
	}

	nc.recompile()
	return proposals
}

// isConsoleCoveredLocked checks if a message is already covered by existing rules (caller holds lock)
func (nc *NoiseConfig) isConsoleCoveredLocked(msg string, entries []LogEntry) bool {
	for i := range nc.compiled {
		if nc.compiled[i].rule.Category != "console" {
			continue
		}
		if nc.compiled[i].messageRegex != nil && nc.compiled[i].messageRegex.MatchString(msg) {
			return true
		}
	}
	// Also check if the source of those entries is already covered
	for _, entry := range entries {
		entryMsg, _ := entry["message"].(string)
		if entryMsg != msg {
			continue
		}
		source, _ := entry["source"].(string)
		for i := range nc.compiled {
			if nc.compiled[i].rule.Category != "console" {
				continue
			}
			if nc.compiled[i].sourceRegex != nil && nc.compiled[i].sourceRegex.MatchString(source) {
				return true
			}
		}
	}
	return false
}

// isSourceCoveredLocked checks if a source is already covered by existing rules (caller holds lock)
func (nc *NoiseConfig) isSourceCoveredLocked(source string) bool {
	for i := range nc.compiled {
		if nc.compiled[i].rule.Category != "console" {
			continue
		}
		if nc.compiled[i].sourceRegex != nil && nc.compiled[i].sourceRegex.MatchString(source) {
			return true
		}
	}
	return false
}

// isURLCoveredLocked checks if a URL path is already covered by existing rules (caller holds lock)
func (nc *NoiseConfig) isURLCoveredLocked(path string) bool {
	for i := range nc.compiled {
		if nc.compiled[i].rule.Category != "network" {
			continue
		}
		if nc.compiled[i].urlRegex != nil && nc.compiled[i].urlRegex.MatchString(path) {
			return true
		}
	}
	return false
}
