// eval_test.go — Tier 1 unit eval runner for all hooks.

package eval

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestEval_AllFixtures(t *testing.T) {
	testdataDir := filepath.Join("testdata")

	fixtures, err := LoadFixtures(testdataDir)
	if err != nil {
		t.Fatalf("LoadFixtures: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no fixtures found")
	}

	// Resolve placeholder paths in fixtures.
	absTestdata, _ := filepath.Abs(testdataDir)
	resolvePlaceholders(fixtures, absTestdata)

	for _, fix := range fixtures {
		fix := fix
		t.Run(fix.Hook+"/"+fix.Description, func(t *testing.T) {
			t.Parallel()
			result := RunFixture(fix, absTestdata)
			if !result.Passed {
				for _, f := range result.Failures {
					t.Error(f)
				}
				if result.Output != "" {
					t.Logf("Output: %s", truncate(result.Output, 500))
				}
			}
			t.Logf("Latency: %dms", result.LatencyMs)
		})
	}
}

func TestEval_Report(t *testing.T) {
	testdataDir := filepath.Join("testdata")

	fixtures, err := LoadFixtures(testdataDir)
	if err != nil {
		t.Fatalf("LoadFixtures: %v", err)
	}

	absTestdata, _ := filepath.Abs(testdataDir)
	resolvePlaceholders(fixtures, absTestdata)

	var results []*Result
	for _, fix := range fixtures {
		results = append(results, RunFixture(fix, absTestdata))
	}

	report := Aggregate(results)
	t.Log("\n" + FormatReport(report))

	if report.Failed > 0 {
		t.Errorf("%d/%d fixtures failed", report.Failed, report.Total)
	}
}

// resolvePlaceholders replaces PLACEHOLDER_* in fixture inputs with real paths.
func resolvePlaceholders(fixtures []*Fixture, testdataDir string) {
	placeholders := map[string]string{
		"PLACEHOLDER_HANDLERS_GO":       filepath.Join(testdataDir, "codebase-go-web", "handlers", "handlers.go"),
		"PLACEHOLDER_LARGE_HANDLER_GO":  filepath.Join(testdataDir, "codebase-go-web", "handlers", "large_handler.go"),
		"PLACEHOLDER_DB_GO":             filepath.Join(testdataDir, "codebase-go-web", "db", "db.go"),
		"PLACEHOLDER_ROUTES_GO":         filepath.Join(testdataDir, "codebase-go-web", "routes", "routes.go"),
		"PLACEHOLDER_MAIN_GO":           filepath.Join(testdataDir, "codebase-go-web", "main.go"),
	}

	for _, fix := range fixtures {
		raw := string(fix.Input.ToolInput)
		for placeholder, realPath := range placeholders {
			raw = strings.ReplaceAll(raw, placeholder, realPath)
		}
		fix.Input.ToolInput = json.RawMessage(raw)

		// Also resolve tool_response if it's a string.
		if len(fix.Input.ToolResponse) > 0 {
			resp := string(fix.Input.ToolResponse)
			for placeholder, realPath := range placeholders {
				resp = strings.ReplaceAll(resp, placeholder, realPath)
			}
			fix.Input.ToolResponse = json.RawMessage(resp)
		}
	}
}
