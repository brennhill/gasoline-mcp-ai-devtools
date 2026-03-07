// decision_guard.go — Architectural decision enforcement for PostToolUse hooks.
// Checks edited code against locked decisions in .gasoline/decisions.json.

package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	decisionsDir  = ".gasoline"
	decisionsFile = "decisions.json"
)

// Decision represents a locked architectural decision.
type Decision struct {
	ID       string `json:"id"`
	Rule     string `json:"rule"`
	Pattern  string `json:"pattern"`   // Literal substring match.
	Regex    string `json:"regex"`     // Regex match (prefix with re: in pattern field, or use this field).
	Reason   string `json:"reason"`
	Enforced string `json:"enforced"`  // Date when decision was made.
	Expires  string `json:"expires"`   // Optional expiry date (YYYY-MM-DD).
}

// DecisionGuardResult holds the findings from decision guard analysis.
type DecisionGuardResult struct {
	Context   string
	Decisions []Decision
}

// FormatContext returns the additionalContext string for the hook output.
func (r *DecisionGuardResult) FormatContext() string {
	return r.Context
}

// RunDecisionGuard checks edited code against project decisions.
// Returns nil if no decisions match or no decision file exists.
func RunDecisionGuard(input Input, projectRoot string) *DecisionGuardResult {
	if !isEditTool(input.ToolName) {
		return nil
	}

	fields := input.ParseToolInput()
	filePath := fields.FilePath
	if filePath == "" || projectRoot == "" {
		return nil
	}

	newContent := extractNewContent(input, fields)
	if newContent == "" {
		return nil
	}

	decisions := loadDecisions(projectRoot)
	if len(decisions) == 0 {
		return nil
	}

	var matched []Decision
	for _, d := range decisions {
		if isExpired(d) {
			continue
		}
		if matchesDecision(d, newContent, filePath) {
			matched = append(matched, d)
		}
	}

	if len(matched) == 0 {
		return nil
	}

	return &DecisionGuardResult{
		Context:   formatDecisions(matched),
		Decisions: matched,
	}
}

func loadDecisions(projectRoot string) []Decision {
	path := filepath.Join(projectRoot, decisionsDir, decisionsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var decisions []Decision
	if json.Unmarshal(data, &decisions) != nil {
		return nil
	}
	return decisions
}

func isExpired(d Decision) bool {
	if d.Expires == "" {
		return false
	}
	expiry, err := time.Parse("2006-01-02", d.Expires)
	if err != nil {
		return false
	}
	return time.Now().After(expiry)
}

func matchesDecision(d Decision, content, filePath string) bool {
	// Check regex pattern.
	if d.Regex != "" {
		re, err := regexp.Compile(d.Regex)
		if err != nil {
			return false // Skip invalid regex.
		}
		if re.MatchString(content) {
			return true
		}
	}

	// Check pattern field.
	if d.Pattern != "" {
		// Support "re:" prefix for inline regex.
		if strings.HasPrefix(d.Pattern, "re:") {
			reStr := strings.TrimPrefix(d.Pattern, "re:")
			re, err := regexp.Compile(reStr)
			if err != nil {
				return false
			}
			return re.MatchString(content)
		}
		// Literal substring match.
		return strings.Contains(content, d.Pattern)
	}

	return false
}

func formatDecisions(decisions []Decision) string {
	var b strings.Builder
	b.WriteString("=== ARCHITECTURAL DECISIONS (do not violate) ===")
	for _, d := range decisions {
		fmt.Fprintf(&b, "\n[%s] %s", d.ID, d.Rule)
		if d.Reason != "" {
			fmt.Fprintf(&b, "\n  Reason: %s", d.Reason)
		}
		if d.Enforced != "" {
			fmt.Fprintf(&b, "\n  Enforced: %s", d.Enforced)
		}
	}
	b.WriteString("\n=== END DECISIONS ===")
	b.WriteString("\nDECISION GUARD: Your edit matches a locked architectural decision. Revise to comply.")
	return b.String()
}
