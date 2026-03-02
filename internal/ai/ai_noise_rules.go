// Purpose: Implements manual noise rule lifecycle operations.
// Why: Separates CRUD/validation paths from runtime matching for modularity and testability.
// Docs: docs/features/feature/noise-filtering/index.md

package ai

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ListRules returns a copy of all current rules.
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

	nestedQuantifierPatterns := []string{
		`\+\s*\)?\s*[\+\*\?]`,
		`\*\s*\)?\s*[\+\*\?]`,
		`\?\s*\)?\s*[\+\*\?]`,
		`\}\s*\)?\s*[\+\*\?]`,
	}

	for _, nestedPattern := range nestedQuantifierPatterns {
		if matched, _ := regexp.MatchString(nestedPattern, pattern); matched {
			return fmt.Errorf("regex pattern contains nested quantifiers which can cause performance issues")
		}
	}

	// Invalid syntax is intentionally not rejected here.
	// recompile() skips regexes that fail compilation to preserve backward compatibility.
	return nil
}

// validateAllRulePatterns validates regex patterns in all rules before any are added.
func validateAllRulePatterns(rules []NoiseRule) error {
	fieldNames := []struct {
		label string
		get   func(*NoiseMatchSpec) string
	}{
		{"MessageRegex", func(spec *NoiseMatchSpec) string { return spec.MessageRegex }},
		{"SourceRegex", func(spec *NoiseMatchSpec) string { return spec.SourceRegex }},
		{"URLRegex", func(spec *NoiseMatchSpec) string { return spec.URLRegex }},
	}

	for i := range rules {
		for _, field := range fieldNames {
			pattern := field.get(&rules[i].MatchSpec)
			if pattern == "" {
				continue
			}
			if err := validateRegexPattern(pattern); err != nil {
				return fmt.Errorf("invalid %s in rule: %w", field.label, err)
			}
		}
	}
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
	case "network", "websocket":
		rule.MatchSpec.URLRegex = pattern
	default:
		rule.MatchSpec.MessageRegex = pattern
	}

	nc.rules = append(nc.rules, rule)
	nc.recompile()
	nc.persistRulesLocked()
}
