// redaction.go — Configurable redaction patterns for MCP tool responses.
// Scrubs sensitive data from tool responses before they reach the AI client.
// Uses RE2 regex (Go's regexp package) for guaranteed linear-time matching.
// Thread-safe: the engine is initialized once at startup and reused across requests.
package redaction

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
)

// RedactionPattern represents a single redaction rule.
type RedactionPattern struct {
	Name        string `json:"name"`
	Pattern     string `json:"pattern"`
	Replacement string `json:"replacement,omitempty"`
}

// RedactionConfig represents the JSON configuration file structure.
type RedactionConfig struct {
	Patterns []RedactionPattern `json:"patterns"`
}

// compiledPattern holds a pre-compiled regex and its replacement string.
type compiledPattern struct {
	name        string
	regex       *regexp.Regexp
	replacement string
	validate    func(match string) bool // optional post-match validation (e.g., Luhn)
}

// RedactionEngine applies a set of compiled patterns to text.
// It is safe for concurrent use after construction.
type RedactionEngine struct {
	patterns []compiledPattern
}

// builtinPatterns defines the always-active redaction rules.
var builtinPatterns = []struct {
	name     string
	pattern  string
	validate func(string) bool
}{
	{
		name:    "aws-key",
		pattern: `AKIA[0-9A-Z]{16}`,
	},
	{
		name:    "bearer-token",
		pattern: `Bearer [A-Za-z0-9\-._~+/]+=*`,
	},
	{
		name:    "basic-auth",
		pattern: `Basic [A-Za-z0-9+/]+=*`,
	},
	{
		name:    "jwt",
		pattern: `eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]+`,
	},
	{
		name:    "github-pat",
		pattern: `(ghp_[A-Za-z0-9]{36,}|github_pat_[A-Za-z0-9_]{36,})`,
	},
	{
		name:    "private-key",
		pattern: `-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`,
	},
	{
		name:     "credit-card",
		pattern:  `\b([0-9]{4}[- ]?[0-9]{4}[- ]?[0-9]{4}[- ]?[0-9]{4})\b`,
		validate: luhnValidateMatch,
	},
	{
		name:    "ssn",
		pattern: `\b[0-9]{3}-[0-9]{2}-[0-9]{4}\b`,
	},
	{
		name:    "api-key",
		pattern: `(?i)(api[_-]?key|apikey|secret[_-]?key)\s*[:=]\s*\S+`,
	},
	{
		name:    "session-cookie",
		pattern: `(?i)(session|sid|token)\s*=\s*[A-Za-z0-9+/=_-]{16,}`,
	},
}

// NewRedactionEngine creates a new engine with built-in patterns and optional
// custom patterns loaded from the given config file path.
// If configPath is empty or the file cannot be read, only built-in patterns are used.
// Invalid regex patterns in the config file are skipped silently.
func NewRedactionEngine(configPath string) *RedactionEngine {
	engine := &RedactionEngine{}

	// Compile built-in patterns
	for _, bp := range builtinPatterns {
		re, err := regexp.Compile(bp.pattern)
		if err != nil {
			continue // should never happen for built-ins, but be safe
		}
		engine.patterns = append(engine.patterns, compiledPattern{
			name:        bp.name,
			regex:       re,
			replacement: "[REDACTED:" + bp.name + "]",
			validate:    bp.validate,
		})
	}

	// Load custom patterns from config file
	if configPath != "" {
		engine.loadConfig(configPath)
	}

	return engine
}

// loadConfig reads and parses the JSON config file, compiling valid patterns.
func (e *RedactionEngine) loadConfig(path string) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is from trusted config location
	if err != nil {
		return // file not found or unreadable — use built-ins only
	}

	var config RedactionConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return // invalid JSON — use built-ins only
	}

	for _, p := range config.Patterns {
		re, err := regexp.Compile(p.Pattern)
		if err != nil {
			continue // skip invalid regex (e.g., PCRE-only features)
		}

		replacement := p.Replacement
		if replacement == "" {
			replacement = "[REDACTED:" + p.Name + "]"
		}

		e.patterns = append(e.patterns, compiledPattern{
			name:        p.Name,
			regex:       re,
			replacement: replacement,
		})
	}
}

// Redact applies all patterns to the input string and returns the redacted result.
// Thread-safe: compiled regexps in Go are safe for concurrent use.
func (e *RedactionEngine) Redact(input string) string {
	if input == "" {
		return ""
	}

	result := input
	for _, p := range e.patterns {
		if p.validate != nil {
			// For patterns with validation, we need to check each match
			result = p.regex.ReplaceAllStringFunc(result, func(match string) string {
				if p.validate(match) {
					return p.replacement
				}
				return match
			})
		} else {
			result = p.regex.ReplaceAllString(result, p.replacement)
		}
	}
	return result
}

// RedactJSON applies redaction to all text fields within an MCP tool result JSON.
// It parses the JSON, redacts text content in MCPContentBlocks, and re-serializes.
// If the JSON is malformed, returns the input with string-level redaction applied.
func (e *RedactionEngine) RedactJSON(input json.RawMessage) json.RawMessage {
	var result MCPToolResult
	if err := json.Unmarshal(input, &result); err != nil {
		// Fallback: redact the raw JSON string
		redacted := e.Redact(string(input))
		return json.RawMessage(redacted)
	}

	// Redact each text content block
	for i := range result.Content {
		if result.Content[i].Type == "text" {
			result.Content[i].Text = e.Redact(result.Content[i].Text)
		}
	}

	output, err := json.Marshal(result)
	if err != nil {
		// Should never happen, but fallback to raw redaction
		return json.RawMessage(e.Redact(string(input)))
	}
	return json.RawMessage(output)
}

// luhnValid checks if a numeric string passes the Luhn algorithm.
func luhnValid(number string) bool {
	// Strip non-digit characters
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, number)

	if len(digits) < 13 || len(digits) > 19 {
		return false
	}

	sum := 0
	alt := false
	for i := len(digits) - 1; i >= 0; i-- {
		n := int(digits[i] - '0')
		if alt {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		alt = !alt
	}
	return sum%10 == 0
}

// luhnValidateMatch is the validation function used by the credit-card pattern.
func luhnValidateMatch(match string) bool {
	return luhnValid(match)
}
