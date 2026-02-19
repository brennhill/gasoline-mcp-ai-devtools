// test_script.go — Playwright test script generation from captured actions.
package generate

import (
	"fmt"
	"strings"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/reproduction"
)

// TestGenParams are the parsed arguments for generate({format: "test"}).
type TestGenParams struct {
	Format              string `json:"format"`
	TestName            string `json:"test_name"`
	LastN               int    `json:"last_n"`
	BaseURL             string `json:"base_url"`
	AssertNetwork       bool   `json:"assert_network"`
	AssertNoErrors      bool   `json:"assert_no_errors"`
	AssertResponseShape bool   `json:"assert_response_shape"`
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
	hasNavigate := false
	for _, a := range group {
		if a.Type == "navigate" {
			hasNavigate = true
			break
		}
	}

	if hasNavigate {
		b.WriteString("    // Verify page loaded successfully\n")
		b.WriteString("    await expect(page).toHaveTitle(/.+/);\n")
	}

	if params.AssertNoErrors {
		b.WriteString("    // Assert no console errors\n")
		b.WriteString("    const errors = [];\n")
		b.WriteString("    page.on('console', msg => { if (msg.type() === 'error') errors.push(msg.text()); });\n")
		b.WriteString("    expect(errors).toHaveLength(0);\n")
	}

	if params.AssertNetwork {
		b.WriteString("    // Assert no failed network requests\n")
		b.WriteString("    const failedRequests = [];\n")
		b.WriteString("    page.on('requestfailed', req => failedRequests.push(req.url()));\n")
		b.WriteString("    expect(failedRequests).toHaveLength(0);\n")
	}
}

// FilterLastN returns the last n actions, or all if n <= 0.
func FilterLastN(actions []capture.EnhancedAction, n int) []capture.EnhancedAction {
	if n <= 0 || n >= len(actions) {
		return actions
	}
	return actions[len(actions)-n:]
}
