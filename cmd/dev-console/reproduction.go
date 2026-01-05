// reproduction.go — Enhanced reproduction script generation.
// Extends basic Playwright script generation with screenshot insertion,
// fixture generation from captured network data, and visual assertions.
// Design: Options default to false to keep generated scripts clean by default.
// When enabled, screenshots capture state at key points, fixtures mock APIs,
// and visual assertions enable snapshot testing.
package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// ReproductionOptions configures enhanced reproduction script generation
type ReproductionOptions struct {
	ErrorMessage       string `json:"error_message"`
	LastNActions       int    `json:"last_n_actions"`
	BaseURL            string `json:"base_url"`
	IncludeScreenshots bool   `json:"include_screenshots"`
	GenerateFixtures   bool   `json:"generate_fixtures"`
	VisualAssertions   bool   `json:"visual_assertions"`
}

// ReproductionResult contains the generated script and optional fixtures
type ReproductionResult struct {
	Script   string                 `json:"script"`
	Fixtures map[string]interface{} `json:"fixtures,omitempty"`
}

// generateEnhancedPlaywrightScript generates a Playwright test script with optional enhancements
func generateEnhancedPlaywrightScript(actions []EnhancedAction, networkBodies []NetworkBody, opts ReproductionOptions) ReproductionResult {
	result := ReproductionResult{}

	// Determine start URL
	startURL := ""
	if len(actions) > 0 && actions[0].URL != "" {
		startURL = actions[0].URL
		if opts.BaseURL != "" {
			startURL = replaceOrigin(startURL, opts.BaseURL)
		}
	}

	// Build test name
	testName := "reproduction: captured user actions"
	if opts.ErrorMessage != "" {
		name := opts.ErrorMessage
		if len(name) > 80 {
			name = name[:80]
		}
		testName = "reproduction: " + name
	}

	// Generate fixtures from network data if requested
	var fixtures map[string]interface{}
	if opts.GenerateFixtures {
		fixtures = generateFixtures(networkBodies)
		result.Fixtures = fixtures
	}

	var sb strings.Builder

	// Write imports
	sb.WriteString("import { test, expect } from '@playwright/test';\n\n")

	// Write fixtures require if needed
	if opts.GenerateFixtures && len(fixtures) > 0 {
		sb.WriteString("// Load fixtures for API mocking\n")
		sb.WriteString("const fixtures = require('./fixtures/api-responses.json');\n\n")
	}

	// Write test function
	sb.WriteString(fmt.Sprintf("test('%s', async ({ page }) => {\n", escapeJSString(testName)))

	// Add route handlers for fixtures
	if opts.GenerateFixtures && len(fixtures) > 0 {
		sb.WriteString("  // Mock API responses with fixtures\n")
		for key := range fixtures {
			sb.WriteString(fmt.Sprintf("  await page.route('**/%s', route => {\n", escapeJSString(key)))
			sb.WriteString(fmt.Sprintf("    route.fulfill({ json: fixtures['%s'] });\n", escapeJSString(key)))
			sb.WriteString("  });\n")
		}
		sb.WriteString("\n")
	}

	// Add navigation
	if startURL != "" {
		sb.WriteString(fmt.Sprintf("  await page.goto('%s');\n", escapeJSString(startURL)))
		if opts.IncludeScreenshots {
			sb.WriteString("  await page.screenshot({ path: 'step-1-navigation.png' });\n")
		}
		if opts.VisualAssertions {
			sb.WriteString("  await expect(page).toHaveScreenshot('navigation.png');\n")
		}
		sb.WriteString("\n")
	}

	// Generate steps with screenshots
	stepNum := 2
	var prevTimestamp int64

	for i := range actions {
		action := &actions[i]

		// Add pause comment for gaps > 2 seconds
		if prevTimestamp > 0 && action.Timestamp-prevTimestamp > 2000 {
			gap := (action.Timestamp - prevTimestamp) / 1000
			sb.WriteString(fmt.Sprintf("  // [%ds pause]\n", gap))
		}
		prevTimestamp = action.Timestamp

		locator := getPlaywrightLocator(action.Selectors)
		actionGenerated := false

		switch action.Type {
		case "click":
			if locator != "" {
				sb.WriteString(fmt.Sprintf("  await page.%s.click();\n", locator))
				actionGenerated = true
			} else {
				sb.WriteString("  // click action - no selector available\n")
			}
		case "input":
			value := action.Value
			if value == "[redacted]" {
				value = "[user-provided]"
			}
			if locator != "" {
				sb.WriteString(fmt.Sprintf("  await page.%s.fill('%s');\n", locator, escapeJSString(value)))
				actionGenerated = true
			}
		case "keypress":
			sb.WriteString(fmt.Sprintf("  await page.keyboard.press('%s');\n", escapeJSString(action.Key)))
			actionGenerated = true
		case "navigate":
			toURL := action.ToURL
			if opts.BaseURL != "" && toURL != "" {
				toURL = replaceOrigin(toURL, opts.BaseURL)
			}
			sb.WriteString(fmt.Sprintf("  await page.waitForURL('%s');\n", escapeJSString(toURL)))
			actionGenerated = true

			// Add screenshot/assertion after navigation
			if opts.IncludeScreenshots {
				pageName := extractPageName(toURL)
				sb.WriteString(fmt.Sprintf("  await page.screenshot({ path: 'step-%d-navigation-%s.png' });\n", stepNum, pageName))
				stepNum++
			}
			if opts.VisualAssertions {
				pageName := extractPageName(toURL)
				sb.WriteString(fmt.Sprintf("  await expect(page).toHaveScreenshot('%s.png');\n", pageName))
			}
		case "select":
			if locator != "" {
				sb.WriteString(fmt.Sprintf("  await page.%s.selectOption('%s');\n", locator, escapeJSString(action.SelectedValue)))
				actionGenerated = true
			}
		case "scroll":
			sb.WriteString(fmt.Sprintf("  // User scrolled to y=%d\n", action.ScrollY))
		}

		// Add screenshot after significant actions (click, input, select)
		if opts.IncludeScreenshots && actionGenerated && action.Type != "navigate" {
			actionDesc := action.Type
			if action.Type == "click" && locator != "" {
				// Extract a short description from the locator
				actionDesc = fmt.Sprintf("click-%s", extractLocatorDesc(locator))
			}
			sb.WriteString(fmt.Sprintf("  await page.screenshot({ path: 'step-%d-%s.png' });\n", stepNum, actionDesc))
			stepNum++
		}
	}

	// Add error comment if present
	if opts.ErrorMessage != "" {
		sb.WriteString(fmt.Sprintf("\n  // Error occurred here: %s\n", opts.ErrorMessage))
	}

	sb.WriteString("});\n")

	result.Script = sb.String()

	// Cap output size (50KB)
	if len(result.Script) > 51200 {
		result.Script = result.Script[:51200]
	}

	return result
}

// generateFixtures extracts API response data from network bodies
func generateFixtures(bodies []NetworkBody) map[string]interface{} {
	fixtures := make(map[string]interface{})

	for _, body := range bodies {
		// Only include JSON responses from API endpoints
		if !strings.Contains(body.ContentType, "json") {
			continue
		}
		if body.ResponseBody == "" {
			continue
		}

		// Parse the URL to get the path
		parsedURL, err := url.Parse(body.URL)
		if err != nil {
			continue
		}

		// Create a fixture key from the path
		path := strings.TrimPrefix(parsedURL.Path, "/")
		if path == "" {
			continue
		}

		// Parse the response body
		var responseData interface{}
		if err := json.Unmarshal([]byte(body.ResponseBody), &responseData); err != nil {
			continue
		}

		// Use the API path as the key (e.g., "api/users" -> "api/users")
		fixtures[path] = responseData
	}

	return fixtures
}

// extractPageName extracts a simple page name from a URL for screenshot naming
func extractPageName(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "page"
	}

	path := strings.TrimPrefix(parsedURL.Path, "/")
	if path == "" {
		return "home"
	}

	// Take the last segment of the path
	segments := strings.Split(path, "/")
	name := segments[len(segments)-1]

	// Sanitize for filename
	name = strings.ReplaceAll(name, ".", "-")
	name = strings.ReplaceAll(name, " ", "-")

	if name == "" {
		return "page"
	}

	return name
}

// extractLocatorDesc extracts a short description from a Playwright locator
func extractLocatorDesc(locator string) string {
	// Extract the identifier from locators like getByTestId('submit-btn')
	if strings.HasPrefix(locator, "getByTestId('") {
		id := strings.TrimPrefix(locator, "getByTestId('")
		if idx := strings.Index(id, "'"); idx > 0 {
			return id[:idx]
		}
	}
	if strings.HasPrefix(locator, "getByRole('") {
		// getByRole('button', { name: 'Save' }) -> button
		id := strings.TrimPrefix(locator, "getByRole('")
		if idx := strings.Index(id, "'"); idx > 0 {
			return id[:idx]
		}
	}
	return "action"
}
