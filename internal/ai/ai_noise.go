// ai_noise.go — Noise filtering rules and auto-detection for browser telemetry.
// Provides regex-based match specs that classify console, network, and WebSocket
// entries as noise (third-party scripts, analytics, browser-generated warnings).
// Auto-detection analyzes buffer contents to suggest rules for high-frequency patterns.
// Design: Rules are AND-matched (all fields must match), stored in a mutex-guarded
// slice with a hard cap of 100 rules. Dismissed patterns use a separate quick-match list.
package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/server"
)

// LogEntry is a type alias for server.LogEntry
type LogEntry = server.LogEntry

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

// PersistedNoiseData is the JSON schema for persisted noise rules
type PersistedNoiseData struct {
	Version    int             `json:"version"`
	NextUserID int             `json:"next_user_id"`
	Rules      []NoiseRule     `json:"rules"`
	Statistics NoiseStatistics `json:"statistics,omitempty"`
}

// NoiseConfig manages noise filtering rules and state
// NoiseConfig manages noise filtering rules with dual-mutex concurrency control.
//
// LOCK ORDERING INVARIANT (H-5 documented):
//
//	mu -> statsMu (always acquire mu before statsMu, never the reverse)
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
	store         *SessionStore // nil if no persistence
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

// NewNoiseConfigWithStore creates a new NoiseConfig with SessionStore persistence
func NewNoiseConfigWithStore(store *SessionStore) *NoiseConfig {
	nc := &NoiseConfig{
		store: store,
		stats: NoiseStatistics{
			PerRule: make(map[string]int),
		},
	}

	nc.rules = builtinRules()
	nc.userIDCounter = 0

	// Load persisted user rules if available
	if store != nil {
		nc.loadPersistedRules()
	}

	nc.recompile()
	return nc
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
		`\+\s*\)?\s*[\+\*\?]`, // + followed by optional ) and another quantifier
		`\*\s*\)?\s*[\+\*\?]`, // * followed by optional ) and another quantifier
		`\?\s*\)?\s*[\+\*\?]`, // ? followed by optional ) and another quantifier
		`\}\s*\)?\s*[\+\*\?]`, // {n,m} followed by optional ) and another quantifier
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

	if err := validateAllRulePatterns(rules); err != nil {
		return err
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
	nc.persistRulesLocked()
	return nil
}

// validateAllRulePatterns validates regex patterns in all rules before any are added.
func validateAllRulePatterns(rules []NoiseRule) error {
	fieldNames := []struct {
		label string
		get   func(*NoiseMatchSpec) string
	}{
		{"MessageRegex", func(s *NoiseMatchSpec) string { return s.MessageRegex }},
		{"SourceRegex", func(s *NoiseMatchSpec) string { return s.SourceRegex }},
		{"URLRegex", func(s *NoiseMatchSpec) string { return s.URLRegex }},
	}
	for i := range rules {
		for _, f := range fieldNames {
			pattern := f.get(&rules[i].MatchSpec)
			if pattern == "" {
				continue
			}
			if err := validateRegexPattern(pattern); err != nil {
				return fmt.Errorf("invalid %s in rule: %w", f.label, err)
			}
		}
	}
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
			nc.persistRulesLocked()
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
	nc.userIDCounter = 0
	nc.recompile()
	nc.stats = NoiseStatistics{
		PerRule: make(map[string]int),
	}
	nc.persistRulesLocked()
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
		if cr.rule.MatchSpec.Level != "" && cr.rule.MatchSpec.Level != level {
			continue
		}
		if matchesConsoleRule(cr, message, source) {
			nc.recordMatch(cr.rule.ID)
			return true
		}
	}

	nc.recordSignal()
	return false
}

// matchesConsoleRule returns true if message or source matches the compiled rule (OR logic).
func matchesConsoleRule(cr *compiledRule, message, source string) bool {
	if cr.messageRegex != nil && cr.messageRegex.MatchString(message) {
		return true
	}
	return cr.sourceRegex != nil && cr.sourceRegex.MatchString(source)
}

// IsNetworkNoise checks if a network body matches any noise rule.
// Security invariant: 401/403 responses are NEVER noise.
func (nc *NoiseConfig) IsNetworkNoise(body capture.NetworkBody) bool {
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
		if !matchesNetworkFilters(cr, body) {
			continue
		}
		if matchesNetworkRule(cr, body.URL) {
			nc.recordMatch(cr.rule.ID)
			return true
		}
	}

	nc.recordSignal()
	return false
}

// matchesNetworkFilters returns true if the body passes the rule's method and status filters.
func matchesNetworkFilters(cr *compiledRule, body capture.NetworkBody) bool {
	if cr.rule.MatchSpec.Method != "" && cr.rule.MatchSpec.Method != body.Method {
		return false
	}
	if cr.rule.MatchSpec.StatusMin > 0 && body.Status < cr.rule.MatchSpec.StatusMin {
		return false
	}
	if cr.rule.MatchSpec.StatusMax > 0 && body.Status > cr.rule.MatchSpec.StatusMax {
		return false
	}
	return true
}

// matchesNetworkRule returns true if the URL matches the rule's regex,
// or if no URL regex is set but method/status filters matched.
func matchesNetworkRule(cr *compiledRule, url string) bool {
	if cr.urlRegex != nil {
		return cr.urlRegex.MatchString(url)
	}
	// No URL regex: match if method or status range was specified
	return cr.rule.MatchSpec.Method != "" || cr.rule.MatchSpec.StatusMin > 0
}

// IsWebSocketNoise checks if a WebSocket event matches any noise rule.
func (nc *NoiseConfig) IsWebSocketNoise(event capture.WebSocketEvent) bool {
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
	nc.persistRulesLocked()
}

// loadPersistedRules loads user rules from SessionStore (called during init)
func (nc *NoiseConfig) loadPersistedRules() {
	if nc.store == nil {
		return
	}

	persisted, ok := nc.readPersistedData()
	if !ok {
		return
	}

	validRules := nc.validatePersistedRules(persisted.Rules)
	nc.restoreUserIDCounter(persisted.NextUserID, validRules)

	// Enforce max rules limit
	maxUserRules := maxNoiseRules - len(nc.rules)
	if len(validRules) > maxUserRules {
		fmt.Fprintf(os.Stderr, "noise: truncating %d rules to fit max of %d\n", len(validRules), maxUserRules)
		validRules = validRules[:maxUserRules]
	}

	nc.rules = append(nc.rules, validRules...)
	nc.restoreStatistics(persisted.Statistics)
}

// readPersistedData loads and unmarshals persisted noise data from the store.
func (nc *NoiseConfig) readPersistedData() (PersistedNoiseData, bool) {
	data, err := nc.store.Load("noise", "rules")
	if err != nil || data == nil {
		return PersistedNoiseData{}, false
	}

	var persisted PersistedNoiseData
	if err := json.Unmarshal(data, &persisted); err != nil {
		fmt.Fprintf(os.Stderr, "noise: corrupted persisted rules: %v\n", err)
		return PersistedNoiseData{}, false
	}

	if persisted.Version != 1 {
		fmt.Fprintf(os.Stderr, "noise: unsupported persistence version: %d\n", persisted.Version)
		return PersistedNoiseData{}, false
	}
	return persisted, true
}

// validatePersistedRules filters rules, skipping built-ins and invalid regexes.
func (nc *NoiseConfig) validatePersistedRules(rules []NoiseRule) []NoiseRule {
	valid := []NoiseRule{}
	for _, rule := range rules {
		if strings.HasPrefix(rule.ID, "builtin_") {
			continue
		}
		if !isRuleRegexValid(rule) {
			fmt.Fprintf(os.Stderr, "noise: skipping rule %s: invalid regex\n", rule.ID)
			continue
		}
		valid = append(valid, rule)
	}
	return valid
}

// isRuleRegexValid checks that all regex patterns in a rule compile.
func isRuleRegexValid(rule NoiseRule) bool {
	patterns := []string{rule.MatchSpec.MessageRegex, rule.MatchSpec.SourceRegex, rule.MatchSpec.URLRegex}
	for _, p := range patterns {
		if p == "" {
			continue
		}
		if _, err := regexp.Compile(p); err != nil {
			return false
		}
	}
	return true
}

// restoreUserIDCounter sets the user ID counter from persisted state, handling desync.
func (nc *NoiseConfig) restoreUserIDCounter(nextUserID int, validRules []NoiseRule) {
	nc.userIDCounter = nextUserID - 1
	maxID := nextUserID - 1
	for _, rule := range validRules {
		if strings.HasPrefix(rule.ID, "user_") {
			idStr := strings.TrimPrefix(rule.ID, "user_")
			if id, err := strconv.Atoi(idStr); err == nil && id > maxID {
				maxID = id
			}
		}
	}
	if maxID > nc.userIDCounter {
		nc.userIDCounter = maxID
	}
}

// restoreStatistics restores noise statistics from persisted data.
func (nc *NoiseConfig) restoreStatistics(stats NoiseStatistics) {
	nc.statsMu.Lock()
	if stats.PerRule != nil {
		nc.stats.PerRule = stats.PerRule
	}
	nc.stats.TotalFiltered = stats.TotalFiltered
	nc.stats.LastSignalAt = stats.LastSignalAt
	nc.stats.LastNoiseAt = stats.LastNoiseAt
	nc.statsMu.Unlock()
}

// persistRulesLocked saves user rules to SessionStore (assumes mu is held)
func (nc *NoiseConfig) persistRulesLocked() {
	if nc.store == nil {
		return
	}

	// Filter to only user rules (exclude built-ins)
	userRules := nc.filterUserRulesLocked()

	// Build persisted data
	nc.statsMu.Lock()
	persisted := PersistedNoiseData{
		Version:    1,
		NextUserID: nc.userIDCounter + 1,
		Rules:      userRules,
		Statistics: NoiseStatistics{
			TotalFiltered: nc.stats.TotalFiltered,
			PerRule:       nc.stats.PerRule,
			LastSignalAt:  nc.stats.LastSignalAt,
			LastNoiseAt:   nc.stats.LastNoiseAt,
		},
	}
	nc.statsMu.Unlock()

	// Marshal and save
	data, err := json.Marshal(persisted)
	if err != nil {
		fmt.Fprintf(os.Stderr, "noise: failed to marshal rules: %v\n", err)
		return
	}

	if err := nc.store.Save("noise", "rules", data); err != nil {
		fmt.Fprintf(os.Stderr, "noise: failed to persist rules: %v\n", err)
		return
	}
}

// filterUserRulesLocked extracts non-builtin rules (assumes mu is held)
func (nc *NoiseConfig) filterUserRulesLocked() []NoiseRule {
	var userRules []NoiseRule
	for _, rule := range nc.rules {
		if !strings.HasPrefix(rule.ID, "builtin_") {
			userRules = append(userRules, rule)
		}
	}
	return userRules
}
