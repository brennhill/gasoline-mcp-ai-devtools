// eval_test.go — Tier 1 unit eval runner for all hooks.

package eval

import (
	"os"
	"path/filepath"
	"testing"
)

// findRepoRoot walks up from dir looking for go.mod.
func findRepoRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func TestEval_AllFixtures(t *testing.T) {
	testdataDir := filepath.Join("testdata")

	fixtures, err := LoadFixtures(testdataDir)
	if err != nil {
		t.Fatalf("LoadFixtures: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no fixtures found")
	}

	absTestdata, _ := filepath.Abs(testdataDir)
	repoRoot := findRepoRoot(absTestdata)
	if repoRoot == "" {
		t.Fatal("cannot find repo root (go.mod)")
	}

	for _, fix := range fixtures {
		fix := fix
		t.Run(fix.Hook+"/"+fix.Description, func(t *testing.T) {
			t.Parallel()
			result := RunFixture(fix, repoRoot)
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
	repoRoot := findRepoRoot(absTestdata)
	if repoRoot == "" {
		t.Fatal("cannot find repo root (go.mod)")
	}

	var results []*Result
	for _, fix := range fixtures {
		results = append(results, RunFixture(fix, repoRoot))
	}

	report := Aggregate(results)
	t.Log("\n" + FormatReport(report))

	if report.Failed > 0 {
		t.Errorf("%d/%d fixtures failed", report.Failed, report.Total)
	}
}
