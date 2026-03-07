// convention_detect.go — Detects patterns in edited code and searches the codebase
// for existing usage, so the AI sees how the project already handles that pattern.
// If 2+ instances exist, suggests extracting a shared helper.

package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	maxFilesToScan         = 500
	maxExamplesPerProbe    = 5
	maxConventionsToReport = 3
	maxFileSizeForScan     = 100 * 1024 // 100KB — skip generated/bundled files
	helperThreshold        = 2          // suggest extracting a helper at this many instances
)

// ConventionMatch holds examples of an existing codebase pattern.
type ConventionMatch struct {
	Pattern  string
	Examples []string // "relative/path.go:42: matched line content"
}

// conventionProbes are literal strings that, when found in new code, trigger a codebase search.
// Type declarations (e.g. `type Foo struct`) are also detected via regex — see DetectConventions.
// These are patterns most likely to cause convention drift when introduced without
// checking existing usage. Language-agnostic where possible.
var conventionProbes = []string{
	// Go patterns
	"http.Client{",
	"map[string]func",
	"sync.Mutex",
	"sync.RWMutex",
	"regexp.MustCompile(",
	"os.WriteFile(",
	"os.ReadFile(",
	"json.MarshalIndent(",
	"json.NewEncoder(",
	"json.NewDecoder(",
	"http.HandleFunc(",
	"http.HandlerFunc",
	"fmt.Errorf(",
	"errors.New(",
	"filepath.Walk",
	"exec.Command(",
	// TypeScript/JavaScript patterns
	"new Map<",
	"new Set<",
	"fetch(",
	"addEventListener(",
	"querySelector(",
	"chrome.storage.",
	"chrome.runtime.",
}

// typePattern detects struct declarations that should be checked for duplicates.
var typePattern = regexp.MustCompile(`type\s+(\w+)\s+struct`)

var skipDirs = map[string]bool{
	".git": true, "vendor": true, "node_modules": true,
	"dist": true, "build": true, ".next": true,
	"__pycache__": true, ".cache": true, ".claude": true,
}

// DetectConventions finds patterns in newContent and searches the project for existing usage.
// Returns nil if no conventions found or if newContent is empty.
func DetectConventions(filePath, projectRoot, newContent string) []ConventionMatch {
	if newContent == "" || projectRoot == "" {
		return nil
	}

	ext := filepath.Ext(filePath)
	exts := extensionFamily(ext)

	// Collect patterns to search for.
	var probes []string
	for _, probe := range conventionProbes {
		if strings.Contains(newContent, probe) {
			probes = append(probes, probe)
		}
	}

	// Check for type declarations (duplicate detection).
	for _, m := range typePattern.FindAllStringSubmatch(newContent, -1) {
		if len(m) > 1 {
			probes = append(probes, "type "+m[1]+" struct")
		}
	}

	if len(probes) == 0 {
		return nil
	}

	// Search the project for each probe.
	var results []ConventionMatch
	for _, probe := range probes {
		examples := searchProject(projectRoot, probe, filePath, exts)
		if len(examples) > 0 {
			results = append(results, ConventionMatch{
				Pattern:  probe,
				Examples: examples,
			})
		}
		if len(results) >= maxConventionsToReport {
			break
		}
	}

	return results
}

// searchProject walks the project tree and finds lines containing the search term.
func searchProject(root, term, excludeFile string, exts []string) []string {
	absExclude, _ := filepath.Abs(excludeFile)
	var examples []string
	filesScanned := 0

	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			if skipDirs[d.Name()] || (strings.HasPrefix(d.Name(), ".") && d.Name() != ".") {
				return filepath.SkipDir
			}
			return nil
		}

		if !matchesExtension(path, exts) {
			return nil
		}

		absPath, _ := filepath.Abs(path)
		if absPath == absExclude {
			return nil
		}

		// Skip large/generated files.
		info, err := d.Info()
		if err != nil || info.Size() > maxFileSizeForScan {
			return nil
		}
		if isGenerated(d.Name()) {
			return nil
		}

		filesScanned++
		if filesScanned > maxFilesToScan {
			return filepath.SkipAll
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if strings.Contains(line, term) {
				relPath, _ := filepath.Rel(root, path)
				if relPath == "" {
					relPath = path
				}
				trimmed := strings.TrimSpace(line)
				if len([]rune(trimmed)) > 120 {
					trimmed = string([]rune(trimmed)[:117]) + "..."
				}
				examples = append(examples, fmt.Sprintf("  %s:%d: %s", relPath, i+1, trimmed))
				if len(examples) >= maxExamplesPerProbe {
					return filepath.SkipAll
				}
				break // one example per file
			}
		}

		return nil
	})

	return examples
}

func isGenerated(name string) bool {
	return strings.Contains(name, ".bundled.") ||
		strings.Contains(name, ".min.") ||
		strings.HasSuffix(name, ".map")
}

func matchesExtension(path string, exts []string) bool {
	ext := filepath.Ext(path)
	for _, e := range exts {
		if ext == e {
			return true
		}
	}
	return false
}

func extensionFamily(ext string) []string {
	switch ext {
	case ".go":
		return []string{".go"}
	case ".ts", ".tsx":
		return []string{".ts", ".tsx", ".js", ".jsx"}
	case ".js", ".jsx":
		return []string{".js", ".jsx", ".ts", ".tsx"}
	case ".py":
		return []string{".py"}
	case ".rs":
		return []string{".rs"}
	default:
		return []string{ext}
	}
}

// FormatConventions formats convention matches for additionalContext output.
// If 2+ instances of a pattern exist, suggests extracting a shared helper.
func FormatConventions(matches []ConventionMatch) string {
	if len(matches) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("=== CODEBASE CONVENTIONS (match existing patterns) ===")
	for _, m := range matches {
		n := len(m.Examples)
		fmt.Fprintf(&b, "\n%s (%d existing usage%s):", m.Pattern, n, pluralS(n))
		for _, ex := range m.Examples {
			fmt.Fprintf(&b, "\n%s", ex)
		}
		if n >= helperThreshold {
			b.WriteString("\n  ^ SUGGESTION: Consider extracting a shared helper — this pattern already exists in ")
			b.WriteString(fmt.Sprintf("%d files. Reuse or align with existing code rather than introducing a new variant.", n))
		}
	}
	b.WriteString("\n=== END CONVENTIONS ===")
	return b.String()
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
