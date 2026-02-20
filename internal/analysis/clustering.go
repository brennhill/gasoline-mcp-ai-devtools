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
//  1. Message Similarity: Normalized messages match (UUIDs/URLs/timestamps replaced)
//  2. Stack Frame Similarity: 2+ shared application-level frames (framework frames excluded)
//  3. Temporal Proximity: Errors within 2 seconds of each other
//
// Clustering Rules (in AddError):
//  1. If existing cluster matches 2+ signals: add to cluster
//  2. If error hasn't matched any cluster: create new cluster with next similar error
//  3. When new cluster formed: compare signals with that error only (not all existing clusters)
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
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/types"
)

// Alert is an alias for types.Alert to avoid qualifying everywhere.
type Alert = types.Alert

// ErrorInstance represents a single error received from the extension.
type ErrorInstance struct {
	Message   string
	Stack     string
	Source    string // file:line extracted from first app frame
	Timestamp time.Time
	ErrorType string // TypeError, ReferenceError, etc.
	Severity  string // error, warning
}

// ErrorCluster groups related errors by root cause.
type ErrorCluster struct {
	ID             string
	Representative ErrorInstance
	NormalizedMsg  string
	CommonFrames   []StackFrame
	RootCause      string
	Instances      []ErrorInstance
	InstanceCount  int
	FirstSeen      time.Time
	LastSeen       time.Time
	AffectedFiles  []string
	Severity       string
	alertedAt3     bool // track if we already alerted for this cluster
}

// ClusterManager manages error clustering with session-scoped lifecycle.
type ClusterManager struct {
	mu             sync.RWMutex
	clusters       []*ErrorCluster
	unclustered    []ErrorInstance
	nextID         int
	expiryDuration time.Duration
	pendingAlert   *Alert
	totalErrors    int
}

// ClusterAnalysisResponse is the response for analyze(target: "errors").
type ClusterAnalysisResponse struct {
	Clusters          []ClusterSummary `json:"clusters"`
	UnclusteredErrors int              `json:"unclustered_errors"`
	TotalErrors       int              `json:"total_errors"`
	Summary           string           `json:"summary"`
}

// ClusterSummary is a single cluster in the analysis response.
type ClusterSummary struct {
	ID                string   `json:"id"`
	RepresentativeMsg string   `json:"representative_error"`
	RootCause         string   `json:"root_cause"`
	InstanceCount     int      `json:"instance_count"`
	FirstSeen         string   `json:"first_seen"`
	LastSeen          string   `json:"last_seen"`
	AffectedFiles     []string `json:"affected_components"`
	Severity          string   `json:"severity"`
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
//  1. Parse error: extract stack frames, normalize message
//  2. Filter app frames (exclude framework code)
//  3. For each existing cluster (linear search):
//     a. Count signals (message + frames + temporal)
//     b. If 2+ signals match: add to this cluster, return
//  4. If not matched to existing cluster:
//     a. Wait for 2nd similar error before clustering (prevents single-error clusters)
//     b. When 2nd error arrives: create new cluster
//  5. Enforce caps: stop clustering if 50 clusters exist
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

	frames := parseStack(err.Stack)
	appFr := appFrames(frames)
	normMsg := normalizeErrorMessage(err.Message)

	if cm.tryMatchExistingCluster(err, appFr, normMsg) {
		return
	}
	if cm.tryFormNewCluster(err, appFr, normMsg) {
		return
	}
	cm.addUnclustered(err)
}

// tryMatchExistingCluster attempts to add the error to an existing cluster.
func (cm *ClusterManager) tryMatchExistingCluster(err ErrorInstance, appFr []StackFrame, normMsg string) bool {
	for _, cluster := range cm.clusters {
		if cm.matchesCluster(cluster, err, appFr, normMsg) {
			cm.addToCluster(cluster, err)
			return true
		}
	}
	return false
}

// tryFormNewCluster attempts to form a new cluster from an unclustered error.
func (cm *ClusterManager) tryFormNewCluster(err ErrorInstance, appFr []StackFrame, normMsg string) bool {
	for i, unc := range cm.unclustered {
		signals := cm.countSignals(unc, err, appFr, normMsg)
		if signals < 2 && (signals < 1 || unc.Stack != "" || err.Stack != "") {
			continue
		}
		cluster := cm.createCluster(unc, err, normMsg)
		cm.clusters = append(cm.clusters, cluster)
		cm.removeUnclustered(i)
		cm.enforceClusterCap()
		return true
	}
	return false
}

// removeUnclustered removes an entry by index from the unclustered slice.
func (cm *ClusterManager) removeUnclustered(i int) {
	newUnclustered := make([]ErrorInstance, len(cm.unclustered)-1)
	copy(newUnclustered, cm.unclustered[:i])
	copy(newUnclustered[i:], cm.unclustered[i+1:])
	cm.unclustered = newUnclustered
}

// enforceClusterCap removes the oldest cluster if the cap is exceeded.
func (cm *ClusterManager) enforceClusterCap() {
	if len(cm.clusters) > 50 {
		newClusters := make([]*ErrorCluster, len(cm.clusters)-1)
		copy(newClusters, cm.clusters[1:])
		cm.clusters = newClusters
	}
}

// addUnclustered adds an error to the unclustered list with FIFO eviction.
func (cm *ClusterManager) addUnclustered(err ErrorInstance) {
	cm.unclustered = append(cm.unclustered, err)
	if len(cm.unclustered) > 100 {
		newUnclustered := make([]ErrorInstance, 100)
		copy(newUnclustered, cm.unclustered[len(cm.unclustered)-100:])
		cm.unclustered = newUnclustered
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

// createCluster creates a new cluster from two matching errors.
// Called after confirming 2+ signals match (AddError calls this).
// Analyzes both errors to determine root cause and scope.
//
// Cluster Initialization:
//  1. Find common stack frames between first & second error
//  2. Infer root cause from common frames + normalized message
//  3. Collect affected files from both error stacks
//  4. Initialize cluster with both errors in instances
//  5. Set creation time and lastSeen for cleanup/timeout tracking
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
//  1. Iterate through clusters (linear scan)
//  2. For each cluster: if now - cluster.lastSeen > 5 minutes:
//     a. Keep track of removed cluster IDs
//     b. Delete cluster from map
//  3. Log cleanup stats (optional, for operator visibility)
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

// GetAnalysisResponse builds the response for analyze(target: "errors").
func (cm *ClusterManager) GetAnalysisResponse() ClusterAnalysisResponse {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	clusters := make([]ClusterSummary, 0, len(cm.clusters))
	for _, c := range cm.clusters {
		clusters = append(clusters, ClusterSummary{
			ID:                c.ID,
			RepresentativeMsg: c.Representative.Message,
			RootCause:         c.RootCause,
			InstanceCount:     c.InstanceCount,
			FirstSeen:         c.FirstSeen.UTC().Format(time.RFC3339),
			LastSeen:          c.LastSeen.UTC().Format(time.RFC3339),
			AffectedFiles:     append([]string(nil), c.AffectedFiles...),
			Severity:          c.Severity,
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
		Clusters:          clusters,
		UnclusteredErrors: len(cm.unclustered),
		TotalErrors:       cm.totalErrors,
		Summary:           summary,
	}
}
