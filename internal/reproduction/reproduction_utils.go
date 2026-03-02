package reproduction

import (
	"net/url"
	"strings"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
)

// EscapeJS escapes a string for embedding in JavaScript string literals.
func EscapeJS(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

// RewriteURL replaces the origin of a URL with baseURL.
func RewriteURL(originalURL, baseURL string) string {
	parsed, err := url.Parse(originalURL)
	if err != nil {
		return originalURL
	}
	return strings.TrimRight(baseURL, "/") + parsed.Path
}

// ChopString truncates s to maxLen characters.
func ChopString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}

// collectSelectorTypes returns the unique selector types present across actions.
func collectSelectorTypes(actions []capture.EnhancedAction) []string {
	types := make(map[string]bool)
	for _, a := range actions {
		if a.Selectors == nil {
			continue
		}
		for key := range a.Selectors {
			types[key] = true
		}
	}
	var result []string
	for t := range types {
		result = append(result, t)
	}
	return result
}
