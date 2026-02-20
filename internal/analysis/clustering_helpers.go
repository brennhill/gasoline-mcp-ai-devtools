// clustering_helpers.go — Stack frame parsing, message normalization, and signal matching for error clustering.

package analysis

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// StackFrame represents a parsed stack trace frame.
type StackFrame struct {
	Function    string
	File        string
	Line        int
	Column      int
	IsFramework bool
}

// Framework path patterns — frames from these paths are excluded from similarity comparison.
var frameworkPatterns = []string{
	"node_modules/react",
	"node_modules/vue",
	"node_modules/@angular",
	"node_modules/svelte",
	"webpack/bootstrap",
	"webpack/runtime",
	"zone.js",
	"node_modules/rxjs",
	"node_modules/core-js",
}

var (
	// "    at FunctionName (file.js:line:col)"
	stackFrameWithFunc = regexp.MustCompile(`^\s*at\s+(.+?)\s+\((.+?):(\d+):(\d+)\)`)
	// "    at file.js:line:col"
	stackFrameAnon = regexp.MustCompile(`^\s*at\s+(.+?):(\d+):(\d+)\s*$`)
)

// parseStackFrame parses a single stack trace line into a StackFrame.
func parseStackFrame(line string) StackFrame {
	line = strings.TrimSpace(line)

	// Try "at Function (file:line:col)"
	if m := stackFrameWithFunc.FindStringSubmatch(line); m != nil {
		lineNum, _ := strconv.Atoi(m[3])
		colNum, _ := strconv.Atoi(m[4])
		frame := StackFrame{
			Function: m[1],
			File:     m[2],
			Line:     lineNum,
			Column:   colNum,
		}
		frame.IsFramework = isFrameworkPath(frame.File)
		return frame
	}

	// Try "at file:line:col" (anonymous)
	if m := stackFrameAnon.FindStringSubmatch(line); m != nil {
		lineNum, _ := strconv.Atoi(m[2])
		colNum, _ := strconv.Atoi(m[3])
		frame := StackFrame{
			Function: "<anonymous>",
			File:     m[1],
			Line:     lineNum,
			Column:   colNum,
		}
		frame.IsFramework = isFrameworkPath(frame.File)
		return frame
	}

	return StackFrame{Function: "<unknown>", File: line}
}

// isFrameworkPath returns true if the file path matches a known framework pattern.
func isFrameworkPath(path string) bool {
	for _, pattern := range frameworkPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

// parseStack parses a multi-line stack trace into frames.
func parseStack(stack string) []StackFrame {
	if stack == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(stack), "\n")
	frames := make([]StackFrame, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		frames = append(frames, parseStackFrame(line))
	}
	return frames
}

// appFrames returns only non-framework frames.
func appFrames(frames []StackFrame) []StackFrame {
	result := make([]StackFrame, 0)
	for _, f := range frames {
		if !f.IsFramework {
			result = append(result, f)
		}
	}
	return result
}

var (
	clusterUUIDRegex      = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	clusterURLRegex       = regexp.MustCompile(`https?://[^\s"']+`)
	clusterTimestampRegex = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[^\s]*`)
	clusterNumericIDRegex = regexp.MustCompile(`\b\d{3,}\b`) // 3+ digit numbers as IDs
)

// normalizeErrorMessage replaces variable content with placeholders.
func normalizeErrorMessage(msg string) string {
	// Order matters: UUIDs before numeric IDs (UUIDs contain digits)
	result := clusterUUIDRegex.ReplaceAllString(msg, "{uuid}")
	result = clusterURLRegex.ReplaceAllString(result, "{url}")
	result = clusterTimestampRegex.ReplaceAllString(result, "{timestamp}")
	result = clusterNumericIDRegex.ReplaceAllString(result, "{id}")
	return result
}

// matchesCluster checks if an error matches an existing cluster.
func (cm *ClusterManager) matchesCluster(cluster *ErrorCluster, err ErrorInstance, appFr []StackFrame, normMsg string) bool {
	// For errors without stacks, message match alone is sufficient
	if err.Stack == "" && len(cluster.Instances) > 0 && cluster.Instances[0].Stack == "" && cluster.NormalizedMsg == normMsg {
		return true
	}
	return cm.clusterSignalCount(cluster, err, appFr, normMsg) >= 2
}

// clusterSignalCount counts how many of the 3 signals match between an error and a cluster.
func (cm *ClusterManager) clusterSignalCount(cluster *ErrorCluster, err ErrorInstance, appFr []StackFrame, normMsg string) int {
	signals := 0
	if cluster.NormalizedMsg == normMsg {
		signals++
	}
	if len(appFr) > 0 && len(cluster.CommonFrames) > 0 && countSharedFrames(appFr, cluster.CommonFrames) >= 1 {
		signals++
	}
	if !cluster.LastSeen.IsZero() && err.Timestamp.Sub(cluster.LastSeen) < 2*time.Second {
		signals++
	}
	return signals
}

// countSignals counts how many signals match between two errors.
// Core signal matching logic used by matchesCluster decision.
// Returns count of matched signals (0-3).
//
// Three Signals Evaluated:
//  1. Message Signal: normalized messages identical
//  2. Frames Signal: 2+ shared application-level frames (countSharedFrames)
//  3. Temporal Signal: time between errors < 2 seconds (temporal proximity)
//
// Decision Rule (caller's responsibility):
//   - 2+ signals matching → likely same root cause
//   - 1 signal matching → could be coincidence, require more evidence
//   - 0 signals matching → different errors
//
// Note on Signal Independence:
//   - Signals are roughly independent (one signal doesn't imply another)
//   - Message alone could be coincidence (same generic message, different code)
//   - Frame signature alone could be coincidence (different bugs in same function)
//   - Time proximity alone could be coincidence (rapid error bursts from unrelated bugs)
//   - 2+ signals together indicate correlation likely > coincidence
//
// Caller must provide:
//   - existing: Error already in this cluster
//   - new: Error being considered for addition
//   - newAppFr: Application frames from new error (already filtered via appFrames())
//   - newNormMsg: Normalized message from new error (already normalized)
func (cm *ClusterManager) countSignals(existing, new ErrorInstance, newAppFr []StackFrame, newNormMsg string) int {
	signals := 0

	// Message similarity
	existingNorm := normalizeErrorMessage(existing.Message)
	if existingNorm == newNormMsg {
		signals++
	}

	// Stack similarity
	existingFrames := appFrames(parseStack(existing.Stack))
	if len(existingFrames) > 0 && len(newAppFr) > 0 {
		if countSharedFrames(existingFrames, newAppFr) >= 1 {
			signals++
		}
	}

	// Temporal proximity
	if new.Timestamp.Sub(existing.Timestamp) < 2*time.Second {
		signals++
	}

	return signals
}

// countSharedFrames counts frames that appear in both slices (by file:line).
func countSharedFrames(a, b []StackFrame) int {
	bSet := make(map[string]bool)
	for _, f := range b {
		bSet[fmt.Sprintf("%s:%d", f.File, f.Line)] = true
	}
	count := 0
	for _, f := range a {
		if bSet[fmt.Sprintf("%s:%d", f.File, f.Line)] {
			count++
		}
	}
	return count
}

// findCommonFrames returns frames present in both slices.
func findCommonFrames(a, b []StackFrame) []StackFrame {
	bSet := make(map[string]StackFrame)
	for _, f := range b {
		key := fmt.Sprintf("%s:%d", f.File, f.Line)
		bSet[key] = f
	}
	var common []StackFrame
	for _, f := range a {
		key := fmt.Sprintf("%s:%d", f.File, f.Line)
		if _, ok := bSet[key]; ok {
			common = append(common, f)
		}
	}
	return common
}

// inferRootCause returns the deepest common app-code frame, or the normalized message.
func inferRootCause(commonFrames []StackFrame, normMsg string) string {
	// The deepest frame (first in the list, since stacks go from deepest to shallowest)
	for _, f := range commonFrames {
		if !f.IsFramework {
			if f.Function != "<anonymous>" && f.Function != "<unknown>" {
				return fmt.Sprintf("%s (%s:%d)", f.Function, f.File, f.Line)
			}
			return fmt.Sprintf("%s:%d", f.File, f.Line)
		}
	}
	return normMsg
}

// collectAffectedFiles returns unique source files from two frame sets.
func collectAffectedFiles(a, b []StackFrame) []string {
	seen := make(map[string]bool)
	var files []string
	for _, frames := range [][]StackFrame{a, b} {
		for _, f := range frames {
			if f.File != "" && !f.IsFramework && !seen[f.File] {
				seen[f.File] = true
				files = append(files, f.File)
			}
		}
	}
	return files
}
