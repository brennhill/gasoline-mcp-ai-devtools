// Purpose: Tracks and exposes noise filtering statistics.
// Why: Isolates metrics mutation/read paths from rule matching and management logic.
// Docs: docs/features/feature/noise-filtering/index.md

package noise

import "time"

// recordMatch updates statistics for a matched rule (thread-safe via statsMu).
func (nc *NoiseConfig) recordMatch(ruleID string) {
	nc.statsMu.Lock()
	defer nc.statsMu.Unlock()

	nc.stats.TotalFiltered++
	nc.stats.PerRule[ruleID]++
	nc.stats.LastNoiseAt = time.Now()
}

// recordSignal updates the last signal timestamp (thread-safe via statsMu).
func (nc *NoiseConfig) recordSignal() {
	nc.statsMu.Lock()
	defer nc.statsMu.Unlock()

	nc.stats.LastSignalAt = time.Now()
}

// GetStatistics returns a copy of the current noise statistics.
func (nc *NoiseConfig) GetStatistics() NoiseStatistics {
	nc.statsMu.Lock()
	defer nc.statsMu.Unlock()

	perRule := make(map[string]int, len(nc.stats.PerRule))
	for key, value := range nc.stats.PerRule {
		perRule[key] = value
	}

	return NoiseStatistics{
		TotalFiltered: nc.stats.TotalFiltered,
		PerRule:       perRule,
		LastSignalAt:  nc.stats.LastSignalAt,
		LastNoiseAt:   nc.stats.LastNoiseAt,
	}
}
