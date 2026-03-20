// Purpose: Generates numbered human-readable reproduction steps from captured browser actions.
// Why: Separates the Gasoline-native output format from Playwright script generation.
package reproduction

import (
	"fmt"
	"strings"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
)

// GenerateGasolineScript converts actions to numbered human-readable steps.
func GenerateGasolineScript(actions []capture.EnhancedAction, opts Params) string {
	if len(actions) == 0 {
		return "# No actions captured\n"
	}
	actions = FilterLastN(actions, opts.LastN)

	var b strings.Builder
	writeGasolineHeader(&b, actions, opts)
	writeGasolineSteps(&b, actions, opts)

	if opts.ErrorMessage != "" {
		b.WriteString(fmt.Sprintf("\n# Error: %s\n", opts.ErrorMessage))
	}
	return b.String()
}

func writeGasolineHeader(b *strings.Builder, actions []capture.EnhancedAction, opts Params) {
	startURL := reproStartURL(actions)
	desc := "captured user actions"
	if opts.ErrorMessage != "" {
		desc = ChopString(opts.ErrorMessage, 80)
	}
	fmt.Fprintf(b, "# Reproduction: %s\n", desc)
	fmt.Fprintf(b, "# Captured: %s | %d actions | %s\n\n",
		time.Now().Format(time.RFC3339), len(actions), startURL)
}

func writeGasolineSteps(b *strings.Builder, actions []capture.EnhancedAction, opts Params) {
	stepNum := 0
	var prevTs int64
	for _, action := range actions {
		WritePauseComment(b, prevTs, action.Timestamp, "   [%ds pause]\n")
		prevTs = action.Timestamp

		line := GasolineStep(action, opts)
		if line == "" {
			continue
		}
		stepNum++
		prefix := ""
		if action.Source == "ai" {
			prefix = "(AI) "
		}
		fmt.Fprintf(b, "%d. %s%s\n", stepNum, prefix, line)
	}
}

// WritePauseComment writes a timing pause comment if the gap exceeds 2 seconds.
func WritePauseComment(b *strings.Builder, prevTs, curTs int64, format string) {
	if prevTs > 0 && curTs-prevTs > 2000 {
		gap := (curTs - prevTs) / 1000
		fmt.Fprintf(b, format, gap)
	}
}

// GasolineStep converts a single action to a natural language step.
func GasolineStep(action capture.EnhancedAction, opts Params) string {
	switch action.Type {
	case "navigate":
		return gasolineNavigateStep(action, opts)
	case "click":
		return "Click: " + DescribeElement(action)
	case "input":
		return gasolineInputStep(action)
	case "select":
		return gasolineSelectStep(action)
	case "keypress":
		return "Press: " + action.Key
	case "scroll":
		return fmt.Sprintf("Scroll to: y=%d", action.ScrollY)
	case "scroll_element":
		return "Scroll to element: " + DescribeElement(action)
	case "refresh":
		return "Refresh page"
	case "back":
		return "Navigate back"
	case "forward":
		return "Navigate forward"
	case "new_tab":
		return gasolineNewTabStep(action, opts)
	case "focus":
		return "Focus: " + DescribeElement(action)
	default:
		return ""
	}
}

func gasolineNavigateStep(action capture.EnhancedAction, opts Params) string {
	toURL := action.ToURL
	if toURL == "" {
		return ""
	}
	if opts.BaseURL != "" {
		toURL = rewriteURL(toURL, opts.BaseURL)
	}
	return "Navigate to: " + toURL
}

func gasolineNewTabStep(action capture.EnhancedAction, opts Params) string {
	targetURL := action.URL
	if targetURL == "" {
		return "Open new tab"
	}
	if opts.BaseURL != "" {
		targetURL = rewriteURL(targetURL, opts.BaseURL)
	}
	return "Open new tab: " + targetURL
}

func gasolineInputStep(action capture.EnhancedAction) string {
	value := action.Value
	if value == "[redacted]" {
		value = "[user-provided]"
	}
	return fmt.Sprintf("Type %q into: %s", value, DescribeElement(action))
}

func gasolineSelectStep(action capture.EnhancedAction) string {
	text := action.SelectedText
	if text == "" {
		text = action.SelectedValue
	}
	return fmt.Sprintf("Select %q from: %s", text, DescribeElement(action))
}
