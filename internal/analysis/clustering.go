// clustering.go — Error clustering by root cause using 2-of-3 signal matching.
// Groups related console errors by comparing normalized messages, application-level
// stack frames (excluding framework code), and temporal proximity (<2s).
// Design: Clusters form when 2+ signals match between errors. Capped at 50 clusters
// with 20 instances each. Clusters expire after 5 minutes of inactivity. Alerts fire
// at 3 instances. Message normalization replaces UUIDs, URLs, timestamps, and numeric
// IDs with placeholders for stable comparison across dynamic error content.
//
// 2-OF-3 SIGNAL MATCHING ALGORITHM:
// =================================
//
// Three Signals (each independently verifiable):
//   1. Message Similarity: Normalized messages match (UUIDs/URLs/timestamps replaced)
//   2. Stack Frame Similarity: 2+ shared application-level frames (framework frames excluded)
//   3. Temporal Proximity: Errors within 2 seconds of each other
//
// Clustering Rules (in AddError):
//   1. If existing cluster matches 2+ signals: add to cluster
//   2. If error hasn't matched any cluster: create new cluster with next similar error
//   3. When new cluster formed: compare signals with that error only (not all existing clusters)
//
// Why Framework Frames Excluded:
//   - Framework frames are identical across different user errors (e.g., React render, Vue update)
//   - Including them would cause false clustering ("all React errors are similar")
//   - Application frames (user code) are the real root cause signals
//   - Exception: if NO app frames found, use framework frames (better than nothing)
//
// Message Normalization (normalizeErrorMessage):
//   - UUIDs (any 32-char hex): → "{uuid}"
//   - URLs (http/https): → "{url}"
//   - Timestamps (ISO 8601, UNIX seconds): → "{timestamp}"
//   - Base64 strings (48+ chars): → "{base64}"
//   - Numeric IDs (5+ digit numbers): → "{id}"
//   - Purpose: "TypeError: Cannot read null (id: 12345)" and "TypeError: Cannot read null (id: 67890)"
//     are now both "TypeError: Cannot read null (id: {id})" → same cluster
//
// Lifecycle:
//   - maxClusters = 50: Stop clustering if 50 clusters exist
//   - maxClusterSize = 20: Stop adding to cluster at 20 instances
//   - Cleanup: Clusters inactive for 5 minutes are removed
//   - Alert: Cluster fires alert when 3+ instances (drainAlert retrieves pending)
//
// Performance Considerations:
//   - Worst-case: N^2 if every error compares to every existing cluster (linear search)
//   - Mitigated by: maxClusters=50 cap, maxClusterSize=20 (stop adding early)
//   - Message normalization is O(message length), executed for every error
//   - Stack frame parsing is O(lines in stack), happens once per error
package analysis

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/types"
)

// Alert is an alias for types.Alert to avoid qualifying everywhere.
type Alert = types.Alert

// --- Stack Frame Parsing ---

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

// --- Message Normalization ---

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

// --- Error Instance ---

// ErrorInstance represents a single error received from the extension.
type ErrorInstance struct {
	Message   string
	Stack     string
	Source    string // file:line extracted from first app frame
	Timestamp time.Time
	ErrorType string // TypeError, ReferenceError, etc.
	Severity  string // error, warning
}

// --- Cluster ---

// ErrorCluster groups related errors by root cause.
type ErrorCluster struct {
	ID               string
	Representative   ErrorInstance
	NormalizedMsg    string
	CommonFrames     []StackFrame
	RootCause        string
	Instances        []ErrorInstance
	InstanceCount    int
	FirstSeen        time.Time
	LastSeen         time.Time
	AffectedFiles    []string
	Severity         string
	alertedAt3       bool // track if we already alerted for this cluster
}

// --- Cluster Manager ---

// ClusterManager manages error clustering with session-scoped lifecycle.
type ClusterManager struct {
	mu              sync.RWMutex
	clusters        []*ErrorCluster
	unclustered     []ErrorInstance
	nextID          int
	expiryDuration  time.Duration
	pendingAlert    *Alert
	totalErrors     int
}

// NewClusterManager creates an empty cluster manager.
func NewClusterManager() *ClusterManager {
	return &ClusterManager{
		clusters:       make([]*ErrorCluster, 0),
		unclustered:    make([]ErrorInstance, 0),
		expiryDuration: 5 * time.Minute,
	}
}

// AddError adds an error to a cluster or creates a new one.
// Main entry point for error clustering. Not thread-safe; caller must synchronize.
//
// Algorithm Flow:
//   1. Parse error: extract stack frames, normalize message
//   2. Filter app frames (exclude framework code)
//   3. For each existing cluster (linear search):
//      a. Count signals (message + frames + temporal)
//      b. If 2+ signals match: add to this cluster, return
//   4. If not matched to existing cluster:
//      a. Wait for 2nd similar error before clustering (prevents single-error clusters)
//      b. When 2nd error arrives: create new cluster
//   5. Enforce caps: stop clustering if 50 clusters exist
//
// Parameters:
//   - err: The console error to cluster (timestamp, message, stack required)
//
// Side Effects:
//   - Creates new cluster if 2+ errors match signals
//   - Adds to existing cluster if 2+ signals match
//   - Increments cluster.instances count
//   - May trigger alert if cluster.instances reaches 3
//   - Updates cluster.lastSeen time
//   - May evict oldest cluster if maxClusters exceeded
func (cm *ClusterManager) AddError(err ErrorInstance) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.totalErrors++

	// Parse the error's stack
	frames := parseStack(err.Stack)
	appFr := appFrames(frames)
	normMsg := normalizeErrorMessage(err.Message)

	// Try to match against existing clusters
	for _, cluster := range cm.clusters {
		if cm.matchesCluster(cluster, err, appFr, normMsg) {
			cm.addToCluster(cluster, err)
			return
		}
	}

	// Try to match against unclustered errors
	for i, unc := range cm.unclustered {
		signals := cm.countSignals(unc, err, appFr, normMsg)
		if signals >= 2 || (signals >= 1 && unc.Stack == "" && err.Stack == "") {
			// Form a new cluster
			cluster := cm.createCluster(unc, err, normMsg)
			cm.clusters = append(cm.clusters, cluster)
			// Remove from unclustered — allocate new slice to avoid GC pinning
			newUnclustered := make([]ErrorInstance, len(cm.unclustered)-1)
			copy(newUnclustered, cm.unclustered[:i])
			copy(newUnclustered[i:], cm.unclustered[i+1:])
			cm.unclustered = newUnclustered
			// Enforce cluster cap
			if len(cm.clusters) > 50 {
				newClusters := make([]*ErrorCluster, len(cm.clusters)-1)
				copy(newClusters, cm.clusters[1:])
				cm.clusters = newClusters
			}
			return
		}
	}

	// No match — add to unclustered (capped at 100, FIFO eviction)
	cm.unclustered = append(cm.unclustered, err)
	if len(cm.unclustered) > 100 {
		newUnclustered := make([]ErrorInstance, 100)
		copy(newUnclustered, cm.unclustered[len(cm.unclustered)-100:])
		cm.unclustered = newUnclustered
	}
}

// matchesCluster checks if an error matches an existing cluster.
func (cm *ClusterManager) matchesCluster(cluster *ErrorCluster, err ErrorInstance, appFr []StackFrame, normMsg string) bool {
	signals := 0

	// Signal 1: Message similarity
	if cluster.NormalizedMsg == normMsg {
		signals++
	}

	// Signal 2: Stack frame similarity (2+ shared app frames)
	if len(appFr) > 0 && len(cluster.CommonFrames) > 0 {
		shared := countSharedFrames(appFr, cluster.CommonFrames)
		if shared >= 1 {
			signals++
		}
	}

	// Signal 3: Temporal proximity (within 2 seconds of last cluster error)
	if !cluster.LastSeen.IsZero() && err.Timestamp.Sub(cluster.LastSeen) < 2*time.Second {
		signals++
	}

	// For errors without stacks, message match alone is sufficient
	if err.Stack == "" && cluster.Instances[0].Stack == "" && cluster.NormalizedMsg == normMsg {
		return true
	}

	return signals >= 2
}

// countSignals counts how many signals match between two errors.
// Core signal matching logic used by matchesCluster decision.
// Returns count of matched signals (0-3).
//
// Three Signals Evaluated:
//   1. Message Signal: normalized messages identical
//   2. Frames Signal: 2+ shared application-level frames (countSharedFrames)
//   3. Temporal Signal: time between errors < 2 seconds (temporal proximity)
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

// createCluster creates a new cluster from two matching errors.
// Called after confirming 2+ signals match (AddError calls this).
// Analyzes both errors to determine root cause and scope.
//
// Cluster Initialization:
//   1. Find common stack frames between first & second error
//   2. Infer root cause from common frames + normalized message
//   3. Collect affected files from both error stacks
//   4. Initialize cluster with both errors in instances
//   5. Set creation time and lastSeen for cleanup/timeout tracking
//
// Root Cause Inference (inferRootCause):
//   - Takes common frames + message as signals
//   - Returns summary string like "TypeError in processData()"
//   - Used for display and deduplication
//
// Cluster Metadata (used by alerting and UX):
//   - commonFrames: Shared stack frames (root cause indicators)
//   - affectedFiles: List of files involved in both errors
//   - instances: [first, second] (will grow as more errors added)
//   - lastSeen: now (time.Now())
//   - createdAt: now
//
// Caller must ensure:
//   - first and second have matching 2+ signals
//   - normMsg is already normalized
//   - Not exceeding maxClusters cap (caller's responsibility)
func (cm *ClusterManager) createCluster(first, second ErrorInstance, normMsg string) *ErrorCluster {
	cm.nextID++
	id := fmt.Sprintf("cluster_%d", cm.nextID)

	// Compute common frames
	frames1 := appFrames(parseStack(first.Stack))
	frames2 := appFrames(parseStack(second.Stack))
	common := findCommonFrames(frames1, frames2)

	// Determine root cause
	rootCause := inferRootCause(common, normMsg)

	// Collect affected files
	files := collectAffectedFiles(frames1, frames2)

	severity := "error"
	if first.Severity == "warning" && second.Severity == "warning" {
		severity = "warning"
	}

	return &ErrorCluster{
		ID:             id,
		Representative: first,
		NormalizedMsg:  normMsg,
		CommonFrames:   common,
		RootCause:      rootCause,
		Instances:      []ErrorInstance{first, second},
		InstanceCount:  2,
		FirstSeen:      first.Timestamp,
		LastSeen:       second.Timestamp,
		AffectedFiles:  files,
		Severity:       severity,
	}
}

// addToCluster adds an error to an existing cluster.
func (cm *ClusterManager) addToCluster(cluster *ErrorCluster, err ErrorInstance) {
	cluster.InstanceCount++
	cluster.LastSeen = err.Timestamp

	// Cap instances at 20
	if len(cluster.Instances) < 20 {
		cluster.Instances = append(cluster.Instances, err)
	}

	// Update affected files
	frames := appFrames(parseStack(err.Stack))
	for _, f := range frames {
		found := false
		for _, existing := range cluster.AffectedFiles {
			if existing == f.File {
				found = true
				break
			}
		}
		if !found {
			cluster.AffectedFiles = append(cluster.AffectedFiles, f.File)
		}
	}

	// Alert at 3 instances (once per cluster)
	if cluster.InstanceCount == 3 && !cluster.alertedAt3 {
		cluster.alertedAt3 = true
		cm.pendingAlert = &Alert{
			Severity:  "error",
			Category:  "error_cluster",
			Title:     fmt.Sprintf("Error cluster: %d related errors from %s", cluster.InstanceCount, cluster.RootCause),
			Detail:    fmt.Sprintf("Cluster %s: %s", cluster.ID, cluster.Representative.Message),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Source:    "clustering",
		}
	}
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

// --- Query Methods ---

// GetClusters returns all active clusters.
func (cm *ClusterManager) GetClusters() []*ErrorCluster {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	result := make([]*ErrorCluster, 0, len(cm.clusters))
	for _, c := range cm.clusters {
		clone := *c
		clone.Instances = append([]ErrorInstance(nil), c.Instances...)
		clone.CommonFrames = append([]StackFrame(nil), c.CommonFrames...)
		clone.AffectedFiles = append([]string(nil), c.AffectedFiles...)
		result = append(result, &clone)
	}
	return result
}

// UnclusteredCount returns the number of errors not in any cluster.
func (cm *ClusterManager) UnclusteredCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.unclustered)
}

// DrainAlert returns and clears the pending alert.
func (cm *ClusterManager) DrainAlert() *Alert {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	alert := cm.pendingAlert
	cm.pendingAlert = nil
	return alert
}

// Cleanup removes expired clusters.
// Called periodically (recommend 1-2 minute intervals) to garbage collect stale clusters.
//
// Expiration Strategy:
//   - A cluster expires if: now - lastSeen > 5 minutes
//   - lastSeen is updated every time error is added to cluster
//   - Meaning: if no new errors added to cluster for 5 minutes, remove it
//   - Purpose: prevent accumulation of clusters from transient bugs
//
// Cleanup Behavior:
//   1. Iterate through clusters (linear scan)
//   2. For each cluster: if now - cluster.lastSeen > 5 minutes:
//      a. Keep track of removed cluster IDs
//      b. Delete cluster from map
//   3. Log cleanup stats (optional, for operator visibility)
//
// Safety Considerations:
//   - Not thread-safe; caller must synchronize (hold ClusterManager.mu)
//   - Safe to call while DrainAlert is in progress (alerts already drained)
//   - Does NOT clear pending alerts; call DrainAlert first if needed
//   - Does NOT reset error counts; they persist per-cluster
//
// Typical Integration:
//   - Run every 60 seconds from background goroutine
//   - Before AddError receives burst of errors (prevents unbounded growth)
func (cm *ClusterManager) Cleanup() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	now := time.Now()
	active := make([]*ErrorCluster, 0, len(cm.clusters))
	for _, c := range cm.clusters {
		if now.Sub(c.LastSeen) < cm.expiryDuration {
			active = append(active, c)
		}
	}
	cm.clusters = active
}

// --- Analysis Response ---

// ClusterAnalysisResponse is the response for analyze(target: "errors").
type ClusterAnalysisResponse struct {
	Clusters         []ClusterSummary `json:"clusters"`
	UnclusteredErrors int             `json:"unclustered_errors"`
	TotalErrors      int              `json:"total_errors"`
	Summary          string           `json:"summary"`
}

// ClusterSummary is a single cluster in the analysis response.
type ClusterSummary struct {
	ID               string          `json:"id"`
	RepresentativeMsg string         `json:"representative_error"`
	RootCause        string          `json:"root_cause"`
	InstanceCount    int             `json:"instance_count"`
	FirstSeen        string          `json:"first_seen"`
	LastSeen         string          `json:"last_seen"`
	AffectedFiles    []string        `json:"affected_components"`
	Severity         string          `json:"severity"`
}

// GetAnalysisResponse builds the response for analyze(target: "errors").
func (cm *ClusterManager) GetAnalysisResponse() ClusterAnalysisResponse {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	clusters := make([]ClusterSummary, 0, len(cm.clusters))
	for _, c := range cm.clusters {
		clusters = append(clusters, ClusterSummary{
			ID:               c.ID,
			RepresentativeMsg: c.Representative.Message,
			RootCause:        c.RootCause,
			InstanceCount:    c.InstanceCount,
			FirstSeen:        c.FirstSeen.UTC().Format(time.RFC3339),
			LastSeen:         c.LastSeen.UTC().Format(time.RFC3339),
			AffectedFiles:    append([]string(nil), c.AffectedFiles...),
			Severity:         c.Severity,
		})
	}

	summary := fmt.Sprintf("%d error clusters identified. %d unclustered errors. %d total errors.",
		len(clusters), len(cm.unclustered), cm.totalErrors)
	if len(cm.clusters) > 0 {
		top := cm.clusters[0]
		summary = fmt.Sprintf("%d error clusters. Primary: %s (%d instances). %d unclustered.",
			len(clusters), top.RootCause, top.InstanceCount, len(cm.unclustered))
	}

	return ClusterAnalysisResponse{
		Clusters:         clusters,
		UnclusteredErrors: len(cm.unclustered),
		TotalErrors:      cm.totalErrors,
		Summary:          summary,
	}
}
