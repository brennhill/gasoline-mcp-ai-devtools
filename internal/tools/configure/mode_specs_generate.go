// mode_specs_generate.go — generate tool per-mode parameter specs.
// Docs: docs/features/describe_capabilities.md
package configure

var generateModeSpecs = map[string]modeParamSpec{
	"reproduction": {
		Hint:     "Generate Playwright reproduction script from captured actions/errors",
		Optional: []string{"error_message", "last_n", "base_url", "include_screenshots", "generate_fixtures", "visual_assertions"},
	},
	"test": {
		Hint:     "Generate Playwright test from captured actions",
		Optional: []string{"test_name", "assert_network", "assert_no_errors", "assert_response_shape"},
	},
	"pr_summary": {
		Hint: "Generate PR summary from captured session activity",
	},
	"har": {
		Hint:     "Export captured network traffic as HAR file",
		Optional: []string{"url", "method", "status_min", "status_max"},
	},
	"csp": {
		Hint:     "Generate Content-Security-Policy header from observed resources",
		Optional: []string{"mode", "include_report_uri", "exclude_origins"},
	},
	"sri": {
		Hint:     "Generate Subresource Integrity hashes for scripts/styles",
		Optional: []string{"resource_types", "origins"},
	},
	"sarif": {
		Hint:     "Export errors and violations as SARIF for IDE/CI integration",
		Optional: []string{"scope", "include_passes"},
	},
	"visual_test": {
		Hint:     "Generate visual regression test from annotations",
		Optional: []string{"test_name", "annot_session"},
	},
	"annotation_report": {
		Hint:     "Generate markdown report from annotation session",
		Optional: []string{"annot_session"},
	},
	"annotation_issues": {
		Hint:     "Generate structured issue list from annotations",
		Optional: []string{"annot_session"},
	},
	"test_from_context": {
		Hint:     "Generate test from error/interaction/regression context",
		Optional: []string{"context", "error_id", "include_mocks", "output_format"},
	},
	"test_heal": {
		Hint:     "Analyze or repair broken test selectors",
		Optional: []string{"action", "test_file", "test_dir", "broken_selectors", "auto_apply"},
	},
	"test_classify": {
		Hint:     "Classify test failures by root cause",
		Optional: []string{"action", "failure", "failures"},
	},
}
