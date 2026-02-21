// test_script.go — Playwright test script generation from captured actions.
package generate

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/reproduction"
)

// CapturedNetworkCheck is a concrete network assertion candidate derived
// from captured traffic in the current session.
type CapturedNetworkCheck struct {
	Pattern string
	Status  int
	Method  string
}

// TestGenParams are the parsed arguments for generate({format: "test"}).
type TestGenParams struct {
	Format                string                 `json:"format"`
	TestName              string                 `json:"test_name"`
	LastN                 int                    `json:"last_n"`
	BaseURL               string                 `json:"base_url"`
	AssertNetwork         bool                   `json:"assert_network"`
	AssertNoErrors        bool                   `json:"assert_no_errors"`
	AssertResponseShape   bool                   `json:"assert_response_shape"`
	CapturedTitle         string                 `json:"-"`
	CapturedErrorCount    int                    `json:"-"`
	CapturedErrorSamples  []string               `json:"-"`
	CapturedNetworkChecks []CapturedNetworkCheck `json:"-"`
}

// GenerateTestScript builds a complete Playwright test file from captured actions.
func GenerateTestScript(actions []capture.EnhancedAction, params TestGenParams) string {
	var b strings.Builder

	b.WriteString("import { test, expect } from '@playwright/test';\n\n")
	fmt.Fprintf(&b, "test.describe('%s', () => {\n", reproduction.EscapeJS(params.TestName))

	if len(actions) == 0 {
		b.WriteString("  // reason: no_actions_captured\n")
		b.WriteString("  // hint: Navigate and interact with the browser first, then call generate(test) again.\n")
		b.WriteString("  test('should load page', async ({ page }) => {\n")
		b.WriteString("    // No actions captured — add test steps here\n")
		b.WriteString("    await page.goto('/');\n")
		b.WriteString("    await expect(page).toHaveTitle(/.+/);\n")
		b.WriteString("  });\n")
	} else {
		writeTestSteps(&b, actions, params)
	}

	b.WriteString("});\n")
	return b.String()
}

// writeTestSteps groups actions into logical test blocks and writes them.
func writeTestSteps(b *strings.Builder, actions []capture.EnhancedAction, params TestGenParams) {
	groups := GroupActionsByNavigation(actions)

	for i, group := range groups {
		testLabel := testLabelForGroup(group, i)
		fmt.Fprintf(b, "  test('%s', async ({ page }) => {\n", reproduction.EscapeJS(testLabel))
		writeAssertionSetup(b, params)

		opts := reproduction.Params{BaseURL: params.BaseURL}
		var prevTs int64
		for _, action := range group {
			reproduction.WritePauseComment(b, prevTs, action.Timestamp, "    // [%ds pause]\n")
			prevTs = action.Timestamp
			line := reproduction.PlaywrightStep(action, opts)
			if line != "" {
				b.WriteString("    " + line + "\n")
			}
		}

		writeTestAssertions(b, group, params)

		b.WriteString("  });\n\n")
	}
}

func writeAssertionSetup(b *strings.Builder, params TestGenParams) {
	if params.AssertNoErrors {
		b.WriteString("    const pageErrors = [];\n")
		b.WriteString("    const consoleErrors = [];\n")
		b.WriteString("    page.on('pageerror', err => pageErrors.push(err.message));\n")
		b.WriteString("    page.on('console', msg => { if (msg.type() === 'error') consoleErrors.push(msg.text()); });\n")
	}

	if params.AssertNetwork {
		if len(params.CapturedNetworkChecks) > 0 {
			b.WriteString("    const observedResponses = [];\n")
			b.WriteString("    page.on('response', res => observedResponses.push({ url: res.url(), status: res.status() }));\n")
			return
		}
		b.WriteString("    const failedRequests = [];\n")
		b.WriteString("    page.on('requestfailed', req => failedRequests.push(req.url()));\n")
	}
}

// GroupActionsByNavigation splits actions into groups at each navigate action.
func GroupActionsByNavigation(actions []capture.EnhancedAction) [][]capture.EnhancedAction {
	if len(actions) == 0 {
		return nil
	}
	var groups [][]capture.EnhancedAction
	var current []capture.EnhancedAction

	for _, action := range actions {
		if action.Type == "navigate" && len(current) > 0 {
			groups = append(groups, current)
			current = nil
		}
		current = append(current, action)
	}
	if len(current) > 0 {
		groups = append(groups, current)
	}
	return groups
}

// testLabelForGroup generates a descriptive test label for a group of actions.
func testLabelForGroup(group []capture.EnhancedAction, index int) string {
	if len(group) == 0 {
		return fmt.Sprintf("step %d", index+1)
	}
	first := group[0]
	if first.Type == "navigate" && first.ToURL != "" {
		path := first.ToURL
		if idx := strings.Index(path, "://"); idx >= 0 {
			path = path[idx+3:]
		}
		if idx := strings.Index(path, "/"); idx >= 0 {
			path = path[idx:]
		}
		if path == "/" || path == "" {
			path = "homepage"
		}
		return fmt.Sprintf("should work on %s", reproduction.ChopString(path, 60))
	}
	return fmt.Sprintf("step %d", index+1)
}

// writeTestAssertions adds expect() assertions at the end of a test block.
func writeTestAssertions(b *strings.Builder, group []capture.EnhancedAction, params TestGenParams) {
	navigateURL := latestNavigateURL(group)
	if navigateURL != "" {
		b.WriteString("    // Verify page loaded to captured navigation URL\n")
		fmt.Fprintf(b, "    await expect(page).toHaveURL('%s');\n", reproduction.EscapeJS(navigateURL))
	}
	if strings.TrimSpace(params.CapturedTitle) != "" {
		b.WriteString("    // Verify captured page title\n")
		fmt.Fprintf(b, "    await expect(page).toHaveTitle(/%s/);\n", escapeRegexForJS(params.CapturedTitle))
	}

	if params.AssertNoErrors {
		if params.CapturedErrorCount > 0 {
			fmt.Fprintf(b, "    // Verify no console/page errors (%d errors captured during session)\n", params.CapturedErrorCount)
		} else {
			b.WriteString("    // Verify no console/page errors captured during replay\n")
		}
		if len(params.CapturedErrorSamples) > 0 {
			b.WriteString("    const expectedErrorPatterns = [")
			for i, sample := range params.CapturedErrorSamples {
				if i > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(b, "%q", sample)
			}
			b.WriteString("];\n")
			b.WriteString("    for (const pattern of expectedErrorPatterns) {\n")
			b.WriteString("      expect(pageErrors.join('\\n')).not.toContain(pattern);\n")
			b.WriteString("      expect(consoleErrors.join('\\n')).not.toContain(pattern);\n")
			b.WriteString("    }\n")
		}
		b.WriteString("    expect(pageErrors).toHaveLength(0);\n")
		b.WriteString("    expect(consoleErrors).toHaveLength(0);\n")
	}

	if params.AssertNetwork {
		if len(params.CapturedNetworkChecks) > 0 {
			b.WriteString("    // Assert key captured network requests completed with expected status\n")
			for i, check := range params.CapturedNetworkChecks {
				fmt.Fprintf(b, "    const match%d = observedResponses.find(r => r.url.includes(%q));\n", i, check.Pattern)
				fmt.Fprintf(b, "    expect(match%d).toBeDefined();\n", i)
				if check.Status > 0 {
					fmt.Fprintf(b, "    expect(match%d.status).toBe(%d);\n", i, check.Status)
				}
			}
		} else {
			b.WriteString("    // Assert no failed network requests\n")
			b.WriteString("    expect(failedRequests).toHaveLength(0);\n")
		}
	}
}

func latestNavigateURL(group []capture.EnhancedAction) string {
	for i := len(group) - 1; i >= 0; i-- {
		if group[i].Type != "navigate" {
			continue
		}
		if group[i].ToURL != "" {
			return group[i].ToURL
		}
		if group[i].URL != "" {
			return group[i].URL
		}
	}
	return ""
}

func escapeRegexForJS(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ".+"
	}
	return regexp.QuoteMeta(s)
}

// FilterLastN returns the last n actions, or all if n <= 0.
func FilterLastN(actions []capture.EnhancedAction, n int) []capture.EnhancedAction {
	if n <= 0 || n >= len(actions) {
		return actions
	}
	return actions[len(actions)-n:]
}
