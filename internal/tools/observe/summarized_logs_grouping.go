// Purpose: Groups log entries by fingerprint into recurring groups and anomalies.
// Why: Separates grouping logic from fingerprinting and type definitions.
package observe

import (
	"math"
	"sort"
	"time"
)

// groupLogs performs single-pass grouping of log entries by fingerprint.
// Returns groups (entries with count >= minGroupSize) and anomalies (below threshold).
func groupLogs(entries []logEntryView, minGroupSize int) ([]LogGroup, []LogAnomaly) {
	if len(entries) == 0 {
		return nil, nil
	}

	type pendingSingle struct {
		entry logEntryView
	}

	groups := make(map[string]*LogGroup)
	pending := make(map[string]*pendingSingle)
	var groupOrder []string // preserve insertion order

	for _, e := range entries {
		fp := fingerprintMessage(e.Message)
		if fp == "" {
			continue
		}

		if g, ok := groups[fp]; ok {
			// Existing group — increment
			g.Count++
			g.LevelBreakdown[e.Level]++
			g.SampleMessage = e.Message // most recent
			if e.TS != "" && (g.FirstSeen == "" || e.TS < g.FirstSeen) {
				g.FirstSeen = e.TS
			}
			if e.TS != "" && e.TS > g.LastSeen {
				g.LastSeen = e.TS
			}
			if e.TS != "" {
				g.timestamps = append(g.timestamps, e.TS)
			}
			if e.Source != "" {
				g.sourceCounts[e.Source]++
			}
			continue
		}

		if p, ok := pending[fp]; ok {
			// Second occurrence — promote to group
			g := &LogGroup{
				Fingerprint:    fp,
				SampleMessage:  e.Message,
				Count:          2,
				LevelBreakdown: map[string]int{p.entry.Level: 1, e.Level: 1},
				FirstSeen:      minStr(p.entry.TS, e.TS),
				LastSeen:       maxStr(p.entry.TS, e.TS),
				sourceCounts:   make(map[string]int),
			}
			if p.entry.Level == e.Level {
				g.LevelBreakdown[e.Level] = 2
			}
			if p.entry.TS != "" {
				g.timestamps = append(g.timestamps, p.entry.TS)
			}
			if e.TS != "" {
				g.timestamps = append(g.timestamps, e.TS)
			}
			if p.entry.Source != "" {
				g.sourceCounts[p.entry.Source]++
			}
			if e.Source != "" {
				g.sourceCounts[e.Source]++
			}
			groups[fp] = g
			groupOrder = append(groupOrder, fp)
			delete(pending, fp)
			continue
		}

		// First occurrence — hold in pending
		pending[fp] = &pendingSingle{entry: e}
	}

	// Pending singles → groups (if minGroupSize <= 1) or anomalies
	var anomalies []LogAnomaly
	for fp, p := range pending {
		if minGroupSize <= 1 {
			g := &LogGroup{
				Fingerprint:    fp,
				SampleMessage:  p.entry.Message,
				Count:          1,
				LevelBreakdown: map[string]int{p.entry.Level: 1},
				FirstSeen:      p.entry.TS,
				LastSeen:       p.entry.TS,
				Source:         p.entry.Source,
				sourceCounts:   make(map[string]int),
			}
			if p.entry.TS != "" {
				g.timestamps = append(g.timestamps, p.entry.TS)
			}
			if p.entry.Source != "" {
				g.sourceCounts[p.entry.Source]++
			}
			groups[fp] = g
			groupOrder = append(groupOrder, fp)
		} else {
			anomalies = append(anomalies, logEntryToAnomaly(p.entry))
		}
	}

	// Build result: groups >= minGroupSize go to groups
	var resultGroups []LogGroup
	for _, fp := range groupOrder {
		g := groups[fp]
		if g.Count < minGroupSize {
			continue
		}
		// Resolve sources
		g.Source, g.Sources = resolveSources(g.sourceCounts)
		resultGroups = append(resultGroups, *g)
	}

	// Sort groups by count desc
	sort.Slice(resultGroups, func(i, j int) bool {
		return resultGroups[i].Count > resultGroups[j].Count
	})

	// Sort anomalies by severity desc, then timestamp desc
	sort.Slice(anomalies, func(i, j int) bool {
		ri := LogLevelRank(anomalies[i].Level)
		rj := LogLevelRank(anomalies[j].Level)
		if ri != rj {
			return ri > rj
		}
		return anomalies[i].TS > anomalies[j].TS
	})

	return resultGroups, anomalies
}

// detectPeriodicity checks each group for regular intervals.
func detectPeriodicity(groups []LogGroup) {
	for i := range groups {
		g := &groups[i]
		if len(g.timestamps) < 3 {
			continue
		}
		// Parse and sort timestamps
		times := make([]time.Time, 0, len(g.timestamps))
		for _, ts := range g.timestamps {
			t, err := time.Parse(time.RFC3339, ts)
			if err == nil {
				times = append(times, t)
			}
		}
		if len(times) < 3 {
			continue
		}
		sort.Slice(times, func(a, b int) bool { return times[a].Before(times[b]) })

		// Compute intervals
		intervals := make([]float64, 0, len(times)-1)
		for j := 1; j < len(times); j++ {
			intervals = append(intervals, times[j].Sub(times[j-1]).Seconds())
		}

		meanValue := mean(intervals)
		if meanValue <= 0 {
			continue
		}
		stddevValue := stddev(intervals, meanValue)
		if stddevValue/meanValue < 0.20 {
			g.IsPeriodic = true
			g.PeriodSeconds = math.Round(meanValue*10) / 10 // round to 1 decimal
		}
	}
}

func logEntryToAnomaly(e logEntryView) LogAnomaly {
	return LogAnomaly(e)
}

func resolveSources(counts map[string]int) (primary string, all []string) {
	if len(counts) == 0 {
		return "", nil
	}
	maxCount := 0
	for src, c := range counts {
		all = append(all, src)
		if c > maxCount {
			maxCount = c
			primary = src
		}
	}
	sort.Strings(all)
	if len(all) <= 1 {
		return primary, nil // omit sources array for single source
	}
	return primary, all
}

func minStr(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	if a < b {
		return a
	}
	return b
}

func maxStr(a, b string) string {
	if a > b {
		return a
	}
	return b
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func stddev(vals []float64, meanValue float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sumSq := 0.0
	for _, v := range vals {
		delta := v - meanValue
		sumSq += delta * delta
	}
	return math.Sqrt(sumSq / float64(len(vals)))
}
