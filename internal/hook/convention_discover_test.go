// convention_discover_test.go — Tests for automatic convention discovery.

package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverConventions_GoProject(t *testing.T) {
	t.Parallel()

	// Find the repo root (the real gasoline codebase).
	root := findRepoRoot(t)

	conventions := DiscoverConventions(root, ".go")
	if len(conventions) == 0 {
		t.Fatal("expected discovered conventions for Go codebase, got none")
	}

	t.Logf("discovered %d Go conventions:", len(conventions))
	for _, c := range conventions {
		t.Logf("  %3d files  %s", c.FileCount, c.Pattern)
	}

	// Sanity: should find patterns we know exist in gasoline.
	found := make(map[string]bool)
	for _, c := range conventions {
		found[c.Pattern] = true
	}

	// These are real gasoline patterns that appear in many files.
	wantSome := []string{
		"json.Unmarshal(",
		"json.Marshal(",
	}
	for _, w := range wantSome {
		if !found[w] {
			t.Errorf("expected to discover %q — it's a real pattern in this codebase", w)
		}
	}

	// Noise should be filtered out.
	noisePatterns := []string{
		"t.Fatalf(",
		"t.Errorf(",
		"strings.Contains(",
		"fmt.Sprintf(",
		"filepath.Join(",
		"mu.Lock(",
	}
	for _, n := range noisePatterns {
		if found[n] {
			t.Errorf("noise pattern %q should be filtered, but was discovered", n)
		}
	}
}

func TestDiscoverConventions_TSProject(t *testing.T) {
	t.Parallel()

	root := findRepoRoot(t)

	conventions := DiscoverConventions(root, ".ts")
	if len(conventions) == 0 {
		t.Fatal("expected discovered conventions for TS files, got none")
	}

	t.Logf("discovered %d TS conventions:", len(conventions))
	for _, c := range conventions {
		t.Logf("  %3d files  %s", c.FileCount, c.Pattern)
	}

	// Noise should be filtered.
	found := make(map[string]bool)
	for _, c := range conventions {
		found[c.Pattern] = true
	}
	tsNoise := []string{"Date.now(", "Math.min(", "JSON.stringify(", "console.log("}
	for _, n := range tsNoise {
		if found[n] {
			t.Errorf("noise pattern %q should be filtered, but was discovered", n)
		}
	}
}

func TestDiscoverConventions_Cache(t *testing.T) {
	t.Parallel()

	root := findRepoRoot(t)

	// First call populates cache.
	c1 := DiscoverConventions(root, ".go")

	// Second call should return cached result (same slice).
	c2 := DiscoverConventions(root, ".go")

	if len(c1) != len(c2) {
		t.Fatalf("cache miss: first call returned %d, second returned %d", len(c1), len(c2))
	}

	for i := range c1 {
		if c1[i].Pattern != c2[i].Pattern {
			t.Errorf("cache miss at index %d: %q vs %q", i, c1[i].Pattern, c2[i].Pattern)
		}
	}
}

func TestDiscoverConventions_EmptyDir(t *testing.T) {
	t.Parallel()

	empty := t.TempDir()
	conventions := DiscoverConventions(empty, ".go")
	if len(conventions) != 0 {
		t.Errorf("expected no conventions in empty dir, got %d", len(conventions))
	}
}

func TestDiscoverConventions_SmallProject(t *testing.T) {
	t.Parallel()

	// Build a minimal project where `db.Query(` appears in 3 files.
	root := t.TempDir()
	files := map[string]string{
		"a.go": "package main\nfunc a() { db.Query(\"SELECT 1\") }\n",
		"b.go": "package main\nfunc b() { db.Query(\"SELECT 2\") }\n",
		"c.go": "package main\nfunc c() { db.Query(\"SELECT 3\") }\n",
		"d.go": "package main\nfunc d() { db.Query(\"SELECT 4\"); db.Exec(\"INSERT\") }\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Clear cache so we don't get stale results.
	discoveryCache.mu.Lock()
	delete(discoveryCache.entries, root+"\x00.go")
	discoveryCache.mu.Unlock()

	conventions := DiscoverConventions(root, ".go")

	found := make(map[string]bool)
	for _, c := range conventions {
		found[c.Pattern] = true
		t.Logf("  %d files  %s", c.FileCount, c.Pattern)
	}

	if !found["db.Query("] {
		t.Error("expected to discover db.Query( in 4 files")
	}
	// db.Exec( only appears in 1 file — below threshold.
	if found["db.Exec("] {
		t.Error("db.Exec( appears in 1 file — should be below threshold")
	}
}

func TestDiscoveredProbes_ReturnStrings(t *testing.T) {
	t.Parallel()

	root := findRepoRoot(t)
	probes := DiscoveredProbes(root, ".go")
	if len(probes) == 0 {
		t.Fatal("expected probes, got none")
	}

	// Every probe should end with (.
	for _, p := range probes {
		if !strings.HasSuffix(p, "(") {
			t.Errorf("probe %q should end with (", p)
		}
	}
}

func TestNoiseFiltering_Comprehensive(t *testing.T) {
	t.Parallel()

	// Verify all noise entries match the regex they're supposed to filter.
	for pattern := range goNoise {
		if !goCallSite.MatchString(pattern) {
			t.Errorf("Go noise pattern %q doesn't match goCallSite regex", pattern)
		}
	}
	for pattern := range tsNoise {
		if !tsCallSite.MatchString(pattern) {
			t.Errorf("TS noise pattern %q doesn't match tsCallSite regex", pattern)
		}
	}
}

// findRepoRoot walks up from the test file to find go.mod.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (go.mod)")
		}
		dir = parent
	}
}
