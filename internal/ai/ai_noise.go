// ai_noise.go — Noise filtering rules and auto-detection for browser telemetry.
// Provides regex-based match specs that classify console, network, and WebSocket
// entries as noise (third-party scripts, analytics, browser-generated warnings).
// Auto-detection analyzes buffer contents to suggest rules for high-frequency patterns.
// Design: Rules are AND-matched (all fields must match), stored in a mutex-guarded
// slice with a hard cap of 100 rules. Dismissed patterns use a separate quick-match list.
package ai

import (
	"fmt"
	"regexp"
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
}

