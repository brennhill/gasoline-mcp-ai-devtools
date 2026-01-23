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
func (v *V4Server) AddPerformanceSnapshot(snapshot PerformanceSnapshot) {
	v.mu.Lock()
	defer v.mu.Unlock()

	url := snapshot.URL

	// LRU eviction for snapshots
	if _, exists := v.perfSnapshots[url]; exists {
		v.perfSnapshotOrder = removeFromOrder(v.perfSnapshotOrder, url)
	} else if len(v.perfSnapshotOrder) >= maxPerfSnapshots {
		oldest := v.perfSnapshotOrder[0]
		delete(v.perfSnapshots, oldest)
		v.perfSnapshotOrder = v.perfSnapshotOrder[1:]
	}
	v.perfSnapshots[url] = snapshot
	v.perfSnapshotOrder = append(v.perfSnapshotOrder, url)

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
func (v *V4Server) updateBaseline(snapshot PerformanceSnapshot) {
	url := snapshot.URL
	baseline, exists := v.perfBaselines[url]

	if !exists {
		// LRU eviction for baselines
		if len(v.perfBaselineOrder) >= maxPerfBaselines {
			oldest := v.perfBaselineOrder[0]
			delete(v.perfBaselines, oldest)
			v.perfBaselineOrder = v.perfBaselineOrder[1:]
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
		}
		v.perfBaselines[url] = baseline
		v.perfBaselineOrder = append(v.perfBaselineOrder, url)
		return
	}

	// Remove from order and re-append (LRU touch)
	v.perfBaselineOrder = removeFromOrder(v.perfBaselineOrder, url)
	v.perfBaselineOrder = append(v.perfBaselineOrder, url)

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

	v.perfBaselines[url] = baseline
}

// GetPerformanceSnapshot returns the snapshot for a given URL
func (v *V4Server) GetPerformanceSnapshot(url string) (PerformanceSnapshot, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	s, ok := v.perfSnapshots[url]
	return s, ok
}

// GetLatestPerformanceSnapshot returns the most recently added snapshot
func (v *V4Server) GetLatestPerformanceSnapshot() (PerformanceSnapshot, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if len(v.perfSnapshotOrder) == 0 {
		return PerformanceSnapshot{}, false
	}
	url := v.perfSnapshotOrder[len(v.perfSnapshotOrder)-1]
	return v.perfSnapshots[url], true
}

// GetPerformanceBaseline returns the baseline for a given URL
func (v *V4Server) GetPerformanceBaseline(url string) (PerformanceBaseline, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	b, ok := v.perfBaselines[url]
	return b, ok
}

// DetectRegressions compares a snapshot against its baseline and returns regressions
func (v *V4Server) DetectRegressions(snapshot PerformanceSnapshot, baseline PerformanceBaseline) []PerformanceRegression {
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
func (v *V4Server) FormatPerformanceReport(snapshot PerformanceSnapshot, baseline *PerformanceBaseline) string {
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

// removeFromOrder removes a string from a slice preserving order
func removeFromOrder(order []string, item string) []string {
	for i, v := range order {
		if v == item {
			return append(order[:i], order[i+1:]...)
		}
	}
	return order
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
func (v *V4Server) HandlePerformanceSnapshot(w http.ResponseWriter, r *http.Request) {
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
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
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
		v.perfSnapshots = make(map[string]PerformanceSnapshot)
		v.perfSnapshotOrder = nil
		v.perfBaselines = make(map[string]PerformanceBaseline)
		v.perfBaselineOrder = nil
		v.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"cleared": true,
		})
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (h *MCPHandlerV4) toolCheckPerformance(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(args, &arguments) // Optional args - zero values are acceptable defaults

	var snapshot PerformanceSnapshot
	var found bool
	if arguments.URL != "" {
		snapshot, found = h.v4.GetPerformanceSnapshot(arguments.URL)
	} else {
		snapshot, found = h.v4.GetLatestPerformanceSnapshot()
	}

	if !found {
		result := map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "No performance snapshot available. Navigate to a page to capture one."},
			},
		}
		resultJSON, _ := json.Marshal(result)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
	}

	baseline, baselineFound := h.v4.GetPerformanceBaseline(snapshot.URL)
	var baselinePtr *PerformanceBaseline
	if baselineFound {
		baselinePtr = &baseline
	}

	report := h.v4.FormatPerformanceReport(snapshot, baselinePtr)

	result := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": report},
		},
	}
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}
