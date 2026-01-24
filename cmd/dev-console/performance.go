package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ============================================
// Performance Budget
// ============================================

// AddPerformanceSnapshot stores a performance snapshot and updates baselines
func (v *Capture) AddPerformanceSnapshot(snapshot PerformanceSnapshot) {
	v.mu.Lock()
	defer v.mu.Unlock()

	url := snapshot.URL

	// LRU eviction for snapshots
	if _, exists := v.perf.snapshots[url]; exists {
		v.perf.snapshotOrder = removeFromSlice(v.perf.snapshotOrder, url)
	} else if len(v.perf.snapshotOrder) >= maxPerfSnapshots {
		oldest := v.perf.snapshotOrder[0]
		delete(v.perf.snapshots, oldest)
		v.perf.snapshotOrder = v.perf.snapshotOrder[1:]
	}
	v.perf.snapshots[url] = snapshot
	v.perf.snapshotOrder = append(v.perf.snapshotOrder, url)

	// Update baseline
	v.updateBaseline(snapshot)
}

// avgOptionalFloat computes a simple running average for nullable float64 pointers
func avgOptionalFloat(baseline *float64, snapshot *float64, n float64) *float64 {
	if snapshot == nil {
		return baseline
	}
	if baseline == nil {
		v := *snapshot
		return &v
	}
	v := *baseline*(n-1)/n + *snapshot/n
	return &v
}

// weightedOptionalFloat computes a weighted average for nullable float64 pointers
func weightedOptionalFloat(baseline *float64, snapshot *float64, baseWeight, newWeight float64) *float64 {
	if snapshot == nil {
		return baseline
	}
	if baseline == nil {
		v := *snapshot
		return &v
	}
	v := *baseline*baseWeight + *snapshot*newWeight
	return &v
}

// updateBaseline updates the running average baseline for a URL
func (v *Capture) updateBaseline(snapshot PerformanceSnapshot) {
	url := snapshot.URL
	baseline, exists := v.perf.baselines[url]

	if !exists {
		// LRU eviction for baselines
		if len(v.perf.baselineOrder) >= maxPerfBaselines {
			oldest := v.perf.baselineOrder[0]
			delete(v.perf.baselines, oldest)
			v.perf.baselineOrder = v.perf.baselineOrder[1:]
		}

		// First sample: use snapshot values directly
		baseline = PerformanceBaseline{
			URL:         url,
			SampleCount: 1,
			LastUpdated: snapshot.Timestamp,
			Timing: BaselineTiming{
				DomContentLoaded:       snapshot.Timing.DomContentLoaded,
				Load:                   snapshot.Timing.Load,
				FirstContentfulPaint:   snapshot.Timing.FirstContentfulPaint,
				LargestContentfulPaint: snapshot.Timing.LargestContentfulPaint,
				TimeToFirstByte:        snapshot.Timing.TimeToFirstByte,
				DomInteractive:         snapshot.Timing.DomInteractive,
			},
			Network: BaselineNetwork{
				RequestCount: snapshot.Network.RequestCount,
				TransferSize: snapshot.Network.TransferSize,
			},
			LongTasks: snapshot.LongTasks,
			CLS:       snapshot.CLS,
			Resources: filterTopResources(snapshot.Resources),
		}
		v.perf.baselines[url] = baseline
		v.perf.baselineOrder = append(v.perf.baselineOrder, url)
		return
	}

	// Remove from order and re-append (LRU touch)
	v.perf.baselineOrder = removeFromSlice(v.perf.baselineOrder, url)
	v.perf.baselineOrder = append(v.perf.baselineOrder, url)

	baseline.SampleCount++
	baseline.LastUpdated = snapshot.Timestamp

	if baseline.SampleCount < 5 {
		// Simple average for first few samples
		n := float64(baseline.SampleCount)
		baseline.Timing.DomContentLoaded = baseline.Timing.DomContentLoaded*(n-1)/n + snapshot.Timing.DomContentLoaded/n
		baseline.Timing.Load = baseline.Timing.Load*(n-1)/n + snapshot.Timing.Load/n
		baseline.Timing.TimeToFirstByte = baseline.Timing.TimeToFirstByte*(n-1)/n + snapshot.Timing.TimeToFirstByte/n
		baseline.Timing.DomInteractive = baseline.Timing.DomInteractive*(n-1)/n + snapshot.Timing.DomInteractive/n
		baseline.Network.RequestCount = int(float64(baseline.Network.RequestCount)*(n-1)/n + float64(snapshot.Network.RequestCount)/n)
		baseline.Network.TransferSize = int64(float64(baseline.Network.TransferSize)*(n-1)/n + float64(snapshot.Network.TransferSize)/n)
		baseline.LongTasks.Count = int(float64(baseline.LongTasks.Count)*(n-1)/n + float64(snapshot.LongTasks.Count)/n)
		baseline.LongTasks.TotalBlockingTime = baseline.LongTasks.TotalBlockingTime*(n-1)/n + snapshot.LongTasks.TotalBlockingTime/n
		baseline.LongTasks.Longest = baseline.LongTasks.Longest*(n-1)/n + snapshot.LongTasks.Longest/n
		baseline.Timing.FirstContentfulPaint = avgOptionalFloat(baseline.Timing.FirstContentfulPaint, snapshot.Timing.FirstContentfulPaint, n)
		baseline.Timing.LargestContentfulPaint = avgOptionalFloat(baseline.Timing.LargestContentfulPaint, snapshot.Timing.LargestContentfulPaint, n)
		baseline.CLS = avgOptionalFloat(baseline.CLS, snapshot.CLS, n)
	} else {
		// Weighted average: 80% existing + 20% new
		baseline.Timing.DomContentLoaded = baseline.Timing.DomContentLoaded*0.8 + snapshot.Timing.DomContentLoaded*0.2
		baseline.Timing.Load = baseline.Timing.Load*0.8 + snapshot.Timing.Load*0.2
		baseline.Timing.TimeToFirstByte = baseline.Timing.TimeToFirstByte*0.8 + snapshot.Timing.TimeToFirstByte*0.2
		baseline.Timing.DomInteractive = baseline.Timing.DomInteractive*0.8 + snapshot.Timing.DomInteractive*0.2
		baseline.Network.RequestCount = int(float64(baseline.Network.RequestCount)*0.8 + float64(snapshot.Network.RequestCount)*0.2)
		baseline.Network.TransferSize = int64(float64(baseline.Network.TransferSize)*0.8 + float64(snapshot.Network.TransferSize)*0.2)
		baseline.LongTasks.Count = int(float64(baseline.LongTasks.Count)*0.8 + float64(snapshot.LongTasks.Count)*0.2)
		baseline.LongTasks.TotalBlockingTime = baseline.LongTasks.TotalBlockingTime*0.8 + snapshot.LongTasks.TotalBlockingTime*0.2
		baseline.LongTasks.Longest = baseline.LongTasks.Longest*0.8 + snapshot.LongTasks.Longest*0.2
		baseline.Timing.FirstContentfulPaint = weightedOptionalFloat(baseline.Timing.FirstContentfulPaint, snapshot.Timing.FirstContentfulPaint, 0.8, 0.2)
		baseline.Timing.LargestContentfulPaint = weightedOptionalFloat(baseline.Timing.LargestContentfulPaint, snapshot.Timing.LargestContentfulPaint, 0.8, 0.2)
		baseline.CLS = weightedOptionalFloat(baseline.CLS, snapshot.CLS, 0.8, 0.2)
	}


	// Update resource fingerprint with moving average
	v.updateBaselineResources(&baseline, snapshot.Resources)
	v.perf.baselines[url] = baseline
}

// GetPerformanceSnapshot returns the snapshot for a given URL
func (v *Capture) GetPerformanceSnapshot(url string) (PerformanceSnapshot, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	s, ok := v.perf.snapshots[url]
	return s, ok
}

// GetLatestPerformanceSnapshot returns the most recently added snapshot
func (v *Capture) GetLatestPerformanceSnapshot() (PerformanceSnapshot, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if len(v.perf.snapshotOrder) == 0 {
		return PerformanceSnapshot{}, false
	}
	url := v.perf.snapshotOrder[len(v.perf.snapshotOrder)-1]
	return v.perf.snapshots[url], true
}

// ============================================
// Web Vitals
// ============================================

// WebVitalMetric represents a single web vital metric with its value and assessment
type WebVitalMetric struct {
	Value      *float64 `json:"value"`
	Assessment string   `json:"assessment"` // "good", "needs-improvement", "poor", or "" if no value
}

// WebVitalsResult holds all core web vitals with their assessments
type WebVitalsResult struct {
	FCP      WebVitalMetric `json:"fcp"`
	LCP      WebVitalMetric `json:"lcp"`
	CLS      WebVitalMetric `json:"cls"`
	INP      WebVitalMetric `json:"inp"`
	LoadTime WebVitalMetric `json:"loadTime"`
	URL      string         `json:"url,omitempty"`
}

// assessFCP returns the assessment for First Contentful Paint
// Good: < 1800ms, Poor: >= 3000ms
func assessFCP(value float64) string {
	if value < 1800 {
		return "good"
	}
	if value >= 3000 {
		return "poor"
	}
	return "needs-improvement"
}

// assessLCP returns the assessment for Largest Contentful Paint
// Good: < 2500ms, Poor: >= 4000ms
func assessLCP(value float64) string {
	if value < 2500 {
		return "good"
	}
	if value >= 4000 {
		return "poor"
	}
	return "needs-improvement"
}

// assessCLS returns the assessment for Cumulative Layout Shift
// Good: < 0.1, Poor: >= 0.25
func assessCLS(value float64) string {
	if value < 0.1 {
		return "good"
	}
	if value >= 0.25 {
		return "poor"
	}
	return "needs-improvement"
}

// assessINP returns the assessment for Interaction to Next Paint
// Good: < 200ms, Poor: >= 500ms
func assessINP(value float64) string {
	if value < 200 {
		return "good"
	}
	if value >= 500 {
		return "poor"
	}
	return "needs-improvement"
}

// GetWebVitals returns the current core web vitals from the latest performance snapshot
func (v *Capture) GetWebVitals() WebVitalsResult {
	v.mu.RLock()
	defer v.mu.RUnlock()

	result := WebVitalsResult{}

	if len(v.perf.snapshotOrder) == 0 {
		return result
	}

	url := v.perf.snapshotOrder[len(v.perf.snapshotOrder)-1]
	snapshot := v.perf.snapshots[url]
	result.URL = snapshot.URL

	// FCP
	if snapshot.Timing.FirstContentfulPaint != nil {
		val := *snapshot.Timing.FirstContentfulPaint
		result.FCP.Value = &val
		result.FCP.Assessment = assessFCP(val)
	}

	// LCP
	if snapshot.Timing.LargestContentfulPaint != nil {
		val := *snapshot.Timing.LargestContentfulPaint
		result.LCP.Value = &val
		result.LCP.Assessment = assessLCP(val)
	}

	// CLS
	if snapshot.CLS != nil {
		val := *snapshot.CLS
		result.CLS.Value = &val
		result.CLS.Assessment = assessCLS(val)
	}

	// INP
	if snapshot.Timing.InteractionToNextPaint != nil {
		val := *snapshot.Timing.InteractionToNextPaint
		result.INP.Value = &val
		result.INP.Assessment = assessINP(val)
	}

	// Load time
	if snapshot.Timing.Load > 0 {
		val := snapshot.Timing.Load
		result.LoadTime.Value = &val
	}

	return result
}

// GetPerformanceBaseline returns the baseline for a given URL
func (v *Capture) GetPerformanceBaseline(url string) (PerformanceBaseline, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	b, ok := v.perf.baselines[url]
	return b, ok
}

// DetectRegressions compares a snapshot against its baseline and returns regressions
func (v *Capture) DetectRegressions(snapshot PerformanceSnapshot, baseline PerformanceBaseline) []PerformanceRegression {
	var regressions []PerformanceRegression

	// Load time: >50% increase AND >200ms absolute
	if baseline.Timing.Load > 0 {
		change := snapshot.Timing.Load - baseline.Timing.Load
		pct := change / baseline.Timing.Load * 100
		if pct > 50 && change > 200 {
			regressions = append(regressions, PerformanceRegression{
				Metric: "load", Current: snapshot.Timing.Load, Baseline: baseline.Timing.Load,
				ChangePercent: pct, AbsoluteChange: change,
			})
		}
	}

	// FCP: >50% increase AND >200ms absolute
	if snapshot.Timing.FirstContentfulPaint != nil && baseline.Timing.FirstContentfulPaint != nil && *baseline.Timing.FirstContentfulPaint > 0 {
		change := *snapshot.Timing.FirstContentfulPaint - *baseline.Timing.FirstContentfulPaint
		pct := change / *baseline.Timing.FirstContentfulPaint * 100
		if pct > 50 && change > 200 {
			regressions = append(regressions, PerformanceRegression{
				Metric: "firstContentfulPaint", Current: *snapshot.Timing.FirstContentfulPaint, Baseline: *baseline.Timing.FirstContentfulPaint,
				ChangePercent: pct, AbsoluteChange: change,
			})
		}
	}

	// LCP: >50% increase AND >200ms absolute
	if snapshot.Timing.LargestContentfulPaint != nil && baseline.Timing.LargestContentfulPaint != nil && *baseline.Timing.LargestContentfulPaint > 0 {
		change := *snapshot.Timing.LargestContentfulPaint - *baseline.Timing.LargestContentfulPaint
		pct := change / *baseline.Timing.LargestContentfulPaint * 100
		if pct > 50 && change > 200 {
			regressions = append(regressions, PerformanceRegression{
				Metric: "largestContentfulPaint", Current: *snapshot.Timing.LargestContentfulPaint, Baseline: *baseline.Timing.LargestContentfulPaint,
				ChangePercent: pct, AbsoluteChange: change,
			})
		}
	}

	// Request count: >50% increase AND >5 absolute
	if baseline.Network.RequestCount > 0 {
		change := float64(snapshot.Network.RequestCount - baseline.Network.RequestCount)
		pct := change / float64(baseline.Network.RequestCount) * 100
		if pct > 50 && change > 5 {
			regressions = append(regressions, PerformanceRegression{
				Metric: "requestCount", Current: float64(snapshot.Network.RequestCount), Baseline: float64(baseline.Network.RequestCount),
				ChangePercent: pct, AbsoluteChange: change,
			})
		}
	}

	// Transfer size: >100% increase AND >100KB absolute
	if baseline.Network.TransferSize > 0 {
		change := float64(snapshot.Network.TransferSize - baseline.Network.TransferSize)
		pct := change / float64(baseline.Network.TransferSize) * 100
		if pct > 100 && change > 102400 {
			regressions = append(regressions, PerformanceRegression{
				Metric: "transferSize", Current: float64(snapshot.Network.TransferSize), Baseline: float64(baseline.Network.TransferSize),
				ChangePercent: pct, AbsoluteChange: change,
			})
		}
	}

	// Long tasks: any increase from 0, or >100% increase
	if baseline.LongTasks.Count == 0 && snapshot.LongTasks.Count > 0 {
		regressions = append(regressions, PerformanceRegression{
			Metric: "longTaskCount", Current: float64(snapshot.LongTasks.Count), Baseline: 0,
			ChangePercent: 100, AbsoluteChange: float64(snapshot.LongTasks.Count),
		})
	} else if baseline.LongTasks.Count > 0 {
		change := float64(snapshot.LongTasks.Count - baseline.LongTasks.Count)
		pct := change / float64(baseline.LongTasks.Count) * 100
		if pct > 100 {
			regressions = append(regressions, PerformanceRegression{
				Metric: "longTaskCount", Current: float64(snapshot.LongTasks.Count), Baseline: float64(baseline.LongTasks.Count),
				ChangePercent: pct, AbsoluteChange: change,
			})
		}
	}

	// TBT: >100ms absolute increase
	tbtChange := snapshot.LongTasks.TotalBlockingTime - baseline.LongTasks.TotalBlockingTime
	if tbtChange > 100 {
		pct := 0.0
		if baseline.LongTasks.TotalBlockingTime > 0 {
			pct = tbtChange / baseline.LongTasks.TotalBlockingTime * 100
		}
		regressions = append(regressions, PerformanceRegression{
			Metric: "totalBlockingTime", Current: snapshot.LongTasks.TotalBlockingTime, Baseline: baseline.LongTasks.TotalBlockingTime,
			ChangePercent: pct, AbsoluteChange: tbtChange,
		})
	}

	// CLS: >0.05 absolute increase
	if snapshot.CLS != nil && baseline.CLS != nil {
		clsChange := *snapshot.CLS - *baseline.CLS
		if clsChange > 0.05 {
			pct := 0.0
			if *baseline.CLS > 0 {
				pct = clsChange / *baseline.CLS * 100
			}
			regressions = append(regressions, PerformanceRegression{
				Metric: "cumulativeLayoutShift", Current: *snapshot.CLS, Baseline: *baseline.CLS,
				ChangePercent: pct, AbsoluteChange: clsChange,
			})
		}
	}

	return regressions
}

// FormatPerformanceReport generates a human-readable performance report
func (v *Capture) FormatPerformanceReport(snapshot PerformanceSnapshot, baseline *PerformanceBaseline) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Performance Snapshot: %s\n", snapshot.URL))
	sb.WriteString(fmt.Sprintf("Captured: %s\n\n", snapshot.Timestamp))

	sb.WriteString("### Navigation Timing\n")
	sb.WriteString(fmt.Sprintf("- TTFB: %.0fms\n", snapshot.Timing.TimeToFirstByte))
	if snapshot.Timing.FirstContentfulPaint != nil {
		sb.WriteString(fmt.Sprintf("- First Contentful Paint: %.0fms\n", *snapshot.Timing.FirstContentfulPaint))
	}
	if snapshot.Timing.LargestContentfulPaint != nil {
		sb.WriteString(fmt.Sprintf("- Largest Contentful Paint: %.0fms\n", *snapshot.Timing.LargestContentfulPaint))
	}
	sb.WriteString(fmt.Sprintf("- DOM Interactive: %.0fms\n", snapshot.Timing.DomInteractive))
	sb.WriteString(fmt.Sprintf("- DOM Content Loaded: %.0fms\n", snapshot.Timing.DomContentLoaded))
	sb.WriteString(fmt.Sprintf("- Load: %.0fms\n", snapshot.Timing.Load))

	sb.WriteString("\n### Network\n")
	sb.WriteString(fmt.Sprintf("- Requests: %d\n", snapshot.Network.RequestCount))
	sb.WriteString(fmt.Sprintf("- Transfer Size: %s\n", formatBytes(snapshot.Network.TransferSize)))
	sb.WriteString(fmt.Sprintf("- Decoded Size: %s\n", formatBytes(snapshot.Network.DecodedSize)))

	if len(snapshot.Network.SlowestRequests) > 0 {
		sb.WriteString("\n### Slowest Requests\n")
		for _, req := range snapshot.Network.SlowestRequests {
			sb.WriteString(fmt.Sprintf("- %.0fms %s (%s)\n", req.Duration, req.URL, formatBytes(req.Size)))
		}
	}

	sb.WriteString("\n### Long Tasks\n")
	sb.WriteString(fmt.Sprintf("- Count: %d\n", snapshot.LongTasks.Count))
	sb.WriteString(fmt.Sprintf("- Total Blocking Time: %.0fms\n", snapshot.LongTasks.TotalBlockingTime))

	if baseline != nil {
		regressions := v.DetectRegressions(snapshot, *baseline)
		if len(regressions) > 0 {
			sb.WriteString("\n### ⚠️ Regressions Detected\n")
			for _, r := range regressions {
				sb.WriteString(fmt.Sprintf("- **%s**: %.0f → %.0f (+%.0f%%, +%.0f)\n",
					r.Metric, r.Baseline, r.Current, r.ChangePercent, r.AbsoluteChange))
			}
		} else {
			sb.WriteString("\n### ✅ No Regressions\n")
			sb.WriteString(fmt.Sprintf("Baseline: %d samples\n", baseline.SampleCount))
		}
	} else {
		sb.WriteString("\n### 📊 No Baseline Yet\n")
		sb.WriteString("This is the first snapshot for this URL. A baseline will be built over subsequent loads.\n")
	}

	return sb.String()
}

// formatBytes formats bytes as human-readable string
func formatBytes(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%dB", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(b)/1024)
	}
	return fmt.Sprintf("%.1fMB", float64(b)/(1024*1024))
}

// ============================================
// HTTP Handlers
// ============================================

// HandlePerformanceSnapshot handles GET, POST, and DELETE /performance-snapshot
func (v *Capture) HandlePerformanceSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		urlFilter := r.URL.Query().Get("url")

		var snapshot PerformanceSnapshot
		var found bool
		if urlFilter != "" {
			snapshot, found = v.GetPerformanceSnapshot(urlFilter)
		} else {
			snapshot, found = v.GetLatestPerformanceSnapshot()
		}

		if !found {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"snapshot": nil,
				"baseline": nil,
			})
			return
		}

		baseline, baselineFound := v.GetPerformanceBaseline(snapshot.URL)

		resp := map[string]interface{}{
			"snapshot": snapshot,
		}
		if baselineFound {
			resp["baseline"] = &baseline
		} else {
			resp["baseline"] = nil
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	if r.Method == "POST" {
		r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}

		var snapshot PerformanceSnapshot
		if err := json.Unmarshal(body, &snapshot); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		v.AddPerformanceSnapshot(snapshot)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"received":         true,
			"baseline_updated": true,
		})
		return
	}

	if r.Method == "DELETE" {
		v.mu.Lock()
		v.perf.snapshots = make(map[string]PerformanceSnapshot)
		v.perf.snapshotOrder = nil
		v.perf.baselines = make(map[string]PerformanceBaseline)
		v.perf.baselineOrder = nil
		v.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"cleared": true,
		})
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (h *ToolHandler) toolCheckPerformance(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(args, &arguments) // Optional args - zero values are acceptable defaults

	var snapshot PerformanceSnapshot
	var found bool
	if arguments.URL != "" {
		snapshot, found = h.capture.GetPerformanceSnapshot(arguments.URL)
	} else {
		snapshot, found = h.capture.GetLatestPerformanceSnapshot()
	}

	if !found {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse("No performance snapshot available. Navigate to a page to capture one.")}
	}

	baseline, baselineFound := h.capture.GetPerformanceBaseline(snapshot.URL)
	var baselinePtr *PerformanceBaseline
	if baselineFound {
		baselinePtr = &baseline
	}

	report := h.capture.FormatPerformanceReport(snapshot, baselinePtr)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(report)}
}

func (h *ToolHandler) toolGetWebVitals(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	result := h.capture.GetWebVitals()
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(resultJSON))}
}

func (h *ToolHandler) toolGetCausalDiff(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		URL        string `json:"url"`
		BaselineID string `json:"baseline_id"`
	}
	_ = json.Unmarshal(args, &arguments)

	result := h.capture.GetCausalDiff(arguments.URL, arguments.BaselineID)

	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(string(resultJSON))}
}

// ============================================
// Causal Diffing
// ============================================

const maxResourceFingerprints = 50
const smallResourceThreshold = 1024 // 1KB

// normalizeResourceURL strips query parameters but preserves hash fragments
func normalizeResourceURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	// Find the query string start
	queryIdx := -1
	hashIdx := -1
	for i, ch := range rawURL {
		if ch == '?' && queryIdx == -1 {
			queryIdx = i
		}
		if ch == '#' && hashIdx == -1 {
			hashIdx = i
		}
	}

	if queryIdx == -1 {
		// No query params, return as-is
		return rawURL
	}

	base := rawURL[:queryIdx]
	if hashIdx > queryIdx {
		// Hash comes after query: preserve hash
		base += rawURL[hashIdx:]
	}
	return base
}

// normalizeDynamicAPIPath groups dynamic API paths by the first 2 segments.
// Paths with 3+ segments get a wildcard after the second segment.
func normalizeDynamicAPIPath(path string) string {
	if path == "" {
		return ""
	}

	// Count path segments
	segments := 0
	lastSlash := -1
	secondSlash := -1
	for i, ch := range path {
		if ch == '/' {
			segments++
			if segments == 3 {
				secondSlash = i
			}
			lastSlash = i
		}
	}
	_ = lastSlash

	// If fewer than 3 slashes (meaning <= 2 segments after root), keep as-is
	if secondSlash == -1 {
		return path
	}

	// Group by first 2 segments + wildcard
	return path[:secondSlash] + "/*"
}

// filterTopResources keeps only the top N resources by transfer size
func filterTopResources(resources []ResourceEntry) []ResourceEntry {
	if len(resources) <= maxResourceFingerprints {
		return resources
	}

	// Find top N by size using selection
	// Copy to avoid mutating input
	sorted := make([]ResourceEntry, len(resources))
	copy(sorted, resources)

	// Sort descending by TransferSize (simple insertion sort for bounded N)
	for i := 1; i < len(sorted); i++ {
		j := i
		for j > 0 && sorted[j].TransferSize > sorted[j-1].TransferSize {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
			j--
		}
	}

	return sorted[:maxResourceFingerprints]
}

// aggregateSmallResources replaces resources under the threshold with a summary entry
func aggregateSmallResources(resources []ResourceEntry, threshold int64) []ResourceEntry {
	var large []ResourceEntry
	var smallCount int
	var smallTotal int64

	for _, r := range resources {
		if r.TransferSize < threshold {
			smallCount++
			smallTotal += r.TransferSize
		} else {
			large = append(large, r)
		}
	}

	if smallCount > 0 {
		large = append(large, ResourceEntry{
			URL:          fmt.Sprintf("%d small resources totaling %s", smallCount, formatBytes(smallTotal)),
			Type:         "aggregated",
			TransferSize: smallTotal,
		})
	}

	return large
}

// computeResourceDiff compares baseline and current resource lists, producing categorized changes
func computeResourceDiff(baseline, current []ResourceEntry) ResourceDiff {
	diff := ResourceDiff{
		Added:   []AddedResource{},
		Removed: []RemovedResource{},
		Resized: []ResizedResource{},
		Retimed: []RetimedResource{},
	}

	// Build maps by normalized URL (with dynamic path grouping for fetch/xmlhttprequest)
	type resourceInfo struct {
		entry      ResourceEntry
		normalURL  string
	}

	normalizeKey := func(r ResourceEntry) string {
		normalized := normalizeResourceURL(r.URL)
		if r.Type == "fetch" || r.Type == "xmlhttprequest" {
			// Extract path from full URL for API path grouping
			path := normalized
			// If it's a full URL, extract path
			if idx := indexOfDoubleslash(normalized); idx >= 0 {
				// Find the path portion after host
				pathStart := idx + 2
				for pathStart < len(normalized) && normalized[pathStart] != '/' {
					pathStart++
				}
				if pathStart < len(normalized) {
					path = normalized[pathStart:]
				}
			}
			grouped := normalizeDynamicAPIPath(path)
			if grouped != path {
				// Reconstruct with grouped path
				if idx := indexOfDoubleslash(normalized); idx >= 0 {
					pathStart := idx + 2
					for pathStart < len(normalized) && normalized[pathStart] != '/' {
						pathStart++
					}
					return normalized[:pathStart] + grouped
				}
				return grouped
			}
		}
		return normalized
	}

	baselineMap := make(map[string]ResourceEntry)
	for _, r := range baseline {
		key := normalizeKey(r)
		baselineMap[key] = r
	}

	currentMap := make(map[string]ResourceEntry)
	for _, r := range current {
		key := normalizeKey(r)
		currentMap[key] = r
	}

	// Find added resources (in current but not baseline)
	for key, curr := range currentMap {
		if _, exists := baselineMap[key]; !exists {
			diff.Added = append(diff.Added, AddedResource{
				URL:            curr.URL,
				Type:           curr.Type,
				SizeBytes:      curr.TransferSize,
				DurationMs:     curr.Duration,
				RenderBlocking: curr.RenderBlocking,
			})
		}
	}

	// Find removed resources (in baseline but not current)
	for key, base := range baselineMap {
		if _, exists := currentMap[key]; !exists {
			diff.Removed = append(diff.Removed, RemovedResource{
				URL:       base.URL,
				Type:      base.Type,
				SizeBytes: base.TransferSize,
			})
		}
	}

	// Find resized and retimed resources (in both)
	for key, curr := range currentMap {
		base, exists := baselineMap[key]
		if !exists {
			continue
		}

		// Check resize: exceeding 10% OR 10KB (whichever is smaller)
		sizeDelta := curr.TransferSize - base.TransferSize
		if sizeDelta < 0 {
			sizeDelta = -sizeDelta
		}
		tenPercent := base.TransferSize / 10
		threshold := tenPercent
		if threshold > 10240 { // 10KB
			threshold = 10240
		}
		if sizeDelta > threshold && threshold > 0 {
			diff.Resized = append(diff.Resized, ResizedResource{
				URL:           curr.URL,
				BaselineBytes: base.TransferSize,
				CurrentBytes:  curr.TransferSize,
				DeltaBytes:    curr.TransferSize - base.TransferSize,
			})
		}

		// Check retimed: duration difference > 100ms
		durationDelta := curr.Duration - base.Duration
		if durationDelta < 0 {
			durationDelta = -durationDelta
		}
		if durationDelta > 100 {
			diff.Retimed = append(diff.Retimed, RetimedResource{
				URL:        curr.URL,
				BaselineMs: base.Duration,
				CurrentMs:  curr.Duration,
				DeltaMs:    curr.Duration - base.Duration,
			})
		}
	}

	return diff
}

// indexOfDoubleslash finds the index of "//" in a string
func indexOfDoubleslash(s string) int {
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '/' && s[i+1] == '/' {
			return i
		}
	}
	return -1
}

// computeProbableCause generates a human-readable probable cause summary
func computeProbableCause(diff ResourceDiff, baselineTotalBytes, currentTotalBytes int64) string {
	if len(diff.Added) == 0 && len(diff.Removed) == 0 && len(diff.Resized) == 0 && len(diff.Retimed) == 0 {
		return "No resource changes detected. Regression may be caused by slower backend responses, increased DOM complexity, or browser throttling."
	}

	var parts []string

	// Sum added resources
	if len(diff.Added) > 0 {
		var totalAdded int64
		var blocking []string
		for _, a := range diff.Added {
			totalAdded += a.SizeBytes
			if a.RenderBlocking {
				blocking = append(blocking, fmt.Sprintf("%s", a.URL))
			}
		}
		parts = append(parts, fmt.Sprintf("Added %dKB in new scripts", totalAdded/1024))
		if len(blocking) > 0 {
			blockList := ""
			for i, b := range blocking {
				if i > 0 {
					blockList += ", "
				}
				blockList += b
			}
			parts = append(parts, fmt.Sprintf("%s is render-blocking", blockList))
		}
	}

	// Retimed resources
	for _, r := range diff.Retimed {
		parts = append(parts, fmt.Sprintf("API response %s slowed by %.0fms", r.URL, r.DeltaMs))
	}

	// Payload increase
	if baselineTotalBytes > 0 && currentTotalBytes > baselineTotalBytes {
		pct := float64(currentTotalBytes-baselineTotalBytes) / float64(baselineTotalBytes) * 100
		if pct >= 1 {
			parts = append(parts, fmt.Sprintf("Total payload increased by %.0f%%", pct))
		}
	}

	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ". "
		}
		result += p
	}
	result += "."
	return result
}

// computeRecommendations generates actionable recommendations from the diff
func computeRecommendations(diff ResourceDiff) []string {
	var recs []string

	// Recommend lazy-loading for render-blocking added scripts
	for _, a := range diff.Added {
		if a.RenderBlocking {
			recs = append(recs, fmt.Sprintf("Consider lazy-loading %s (%dKB, render-blocking)", a.URL, a.SizeBytes/1024))
		}
	}

	// Recommend investigating API regressions
	for _, r := range diff.Retimed {
		recs = append(recs, fmt.Sprintf("Investigate API regression on %s (+%.0fms)", r.URL, r.DeltaMs))
	}

	// Recommend reviewing bundle growth
	for _, r := range diff.Resized {
		recs = append(recs, fmt.Sprintf("%s grew by %dKB — review recent imports", r.URL, r.DeltaBytes/1024))
	}

	// Recommend lazy-loading large non-blocking additions
	for _, a := range diff.Added {
		if !a.RenderBlocking && a.SizeBytes > 100000 {
			recs = append(recs, fmt.Sprintf("Consider code-splitting %s (%dKB)", a.URL, a.SizeBytes/1024))
		}
	}

	return recs
}

// GetCausalDiff computes the causal diff for a URL, comparing current snapshot against baseline
func (v *Capture) GetCausalDiff(url string, baselineID string) CausalDiffResult {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// Resolve URL
	if url == "" {
		if len(v.perf.snapshotOrder) == 0 {
			return CausalDiffResult{ProbableCause: "No snapshots available."}
		}
		url = v.perf.snapshotOrder[len(v.perf.snapshotOrder)-1]
	}

	result := CausalDiffResult{URL: url}

	// Get baseline
	baseline, baselineExists := v.perf.baselines[url]
	if !baselineExists {
		result.ProbableCause = "No baseline available for this URL."
		return result
	}

	// Get current snapshot
	snapshot, snapshotExists := v.perf.snapshots[url]
	if !snapshotExists {
		result.ProbableCause = "No current snapshot available for this URL."
		return result
	}

	// Compute timing delta
	result.TimingDelta.LoadMs = snapshot.Timing.Load - baseline.Timing.Load
	if snapshot.Timing.FirstContentfulPaint != nil && baseline.Timing.FirstContentfulPaint != nil {
		result.TimingDelta.FCPMs = *snapshot.Timing.FirstContentfulPaint - *baseline.Timing.FirstContentfulPaint
	}
	if snapshot.Timing.LargestContentfulPaint != nil && baseline.Timing.LargestContentfulPaint != nil {
		result.TimingDelta.LCPMs = *snapshot.Timing.LargestContentfulPaint - *baseline.Timing.LargestContentfulPaint
	}

	// Check if baseline has resource fingerprint
	if len(baseline.Resources) == 0 && len(snapshot.Resources) == 0 {
		result.ProbableCause = "Resource comparison unavailable — baseline predates resource tracking."
		result.ResourceChanges = ResourceDiff{
			Added:   []AddedResource{},
			Removed: []RemovedResource{},
			Resized: []ResizedResource{},
			Retimed: []RetimedResource{},
		}
		return result
	}

	// Compute resource diff
	result.ResourceChanges = computeResourceDiff(baseline.Resources, snapshot.Resources)

	// Compute total transfer sizes for payload comparison
	var baselineTotal int64
	for _, r := range baseline.Resources {
		baselineTotal += r.TransferSize
	}
	var currentTotal int64
	for _, r := range snapshot.Resources {
		currentTotal += r.TransferSize
	}

	result.ProbableCause = computeProbableCause(result.ResourceChanges, baselineTotal, currentTotal)
	result.Recommendations = computeRecommendations(result.ResourceChanges)

	return result
}

// updateBaselineResources updates the resource fingerprint in the baseline with moving average
func (v *Capture) updateBaselineResources(baseline *PerformanceBaseline, snapshotResources []ResourceEntry) {
	if len(snapshotResources) == 0 {
		return
	}

	// Filter to top resources
	filtered := filterTopResources(snapshotResources)

	if len(baseline.Resources) == 0 {
		// First sample: store directly
		baseline.Resources = make([]ResourceEntry, len(filtered))
		copy(baseline.Resources, filtered)
		return
	}

	// Build a map of existing baseline resources by normalized URL
	normalizeKey := func(r ResourceEntry) string {
		return normalizeResourceURL(r.URL)
	}

	baselineMap := make(map[string]int) // normalized URL -> index in baseline.Resources
	for i, r := range baseline.Resources {
		baselineMap[normalizeKey(r)] = i
	}

	n := float64(baseline.SampleCount)

	// Update existing resources with moving average, add new ones
	for _, snap := range filtered {
		key := normalizeKey(snap)
		if idx, exists := baselineMap[key]; exists {
			// Average the size and duration
			if n < 5 {
				baseline.Resources[idx].TransferSize = int64(
					float64(baseline.Resources[idx].TransferSize)*(n-1)/n + float64(snap.TransferSize)/n,
				)
				baseline.Resources[idx].Duration =
					baseline.Resources[idx].Duration*(n-1)/n + snap.Duration/n
			} else {
				baseline.Resources[idx].TransferSize = int64(
					float64(baseline.Resources[idx].TransferSize)*0.8 + float64(snap.TransferSize)*0.2,
				)
				baseline.Resources[idx].Duration =
					baseline.Resources[idx].Duration*0.8 + snap.Duration*0.2
			}
		} else {
			// New resource: add to baseline
			baseline.Resources = append(baseline.Resources, snap)
		}
	}

	// Re-filter if the list grew too large
	if len(baseline.Resources) > maxResourceFingerprints {
		baseline.Resources = filterTopResources(baseline.Resources)
	}
}
