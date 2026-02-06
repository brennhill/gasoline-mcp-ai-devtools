// ai_noise_detect.go — Auto-detection of noise patterns from browser telemetry buffers.
package ai

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
)

// AutoDetect analyzes buffers and proposes noise rules based on frequency and source analysis.
// High-confidence proposals (>= 0.9) are automatically applied.
// Note: This function holds a write lock for the entire analysis. It is designed for
// infrequent manual invocation via the MCP tool, not for hot-path usage.
func (nc *NoiseConfig) AutoDetect(consoleEntries []LogEntry, networkBodies []capture.NetworkBody, wsEvents []capture.WebSocketEvent) []NoiseProposal {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	proposals := make([]NoiseProposal, 0)

	// --- Frequency analysis for console messages ---
	if len(consoleEntries) > 0 {
		msgCounts := make(map[string]int)
		for _, entry := range consoleEntries {
			msg, _ := entry["message"].(string)
			if msg != "" {
				msgCounts[msg]++
			}
		}

		for msg, count := range msgCounts {
			if count < 10 {
				continue
			}

			// Check if already covered by existing rules
			if nc.isConsoleCoveredLocked(msg, consoleEntries) {
				continue
			}

			confidence := 0.7 + float64(count)/100.0
			if confidence > 0.99 {
				confidence = 0.99
			}

			proposal := NoiseProposal{
				Rule: NoiseRule{
					Category:       "console",
					Classification: "repetitive",
					AutoDetected:   true,
					MatchSpec: NoiseMatchSpec{
						MessageRegex: regexp.QuoteMeta(msg),
					},
				},
				Confidence: confidence,
				Reason:     fmt.Sprintf("message repeated %d times", count),
			}
			proposals = append(proposals, proposal)
		}
	}

	// --- Source analysis for console entries ---
	if len(consoleEntries) > 0 {
		sourceCounts := make(map[string][]LogEntry)
		for _, entry := range consoleEntries {
			source, _ := entry["source"].(string)
			if source != "" && strings.Contains(source, "node_modules") {
				// Group by source path
				sourceCounts[source] = append(sourceCounts[source], entry)
			}
		}

		for source, entries := range sourceCounts {
			if len(entries) < 2 {
				continue
			}
			// Check if already covered
			if nc.isSourceCoveredLocked(source) {
				continue
			}

			proposal := NoiseProposal{
				Rule: NoiseRule{
					Category:       "console",
					Classification: "extension",
					AutoDetected:   true,
					MatchSpec: NoiseMatchSpec{
						SourceRegex: regexp.QuoteMeta(source),
					},
				},
				Confidence: 0.75,
				Reason:     fmt.Sprintf("node_modules source with %d entries", len(entries)),
			}
			proposals = append(proposals, proposal)
		}
	}

	// --- Network frequency analysis ---
	if len(networkBodies) > 0 {
		urlCounts := make(map[string]int)
		for _, body := range networkBodies {
			// Extract path from URL
			path := capture.ExtractURLPath(body.URL)
			if path != "" {
				urlCounts[path]++
			}
		}

		infraPatterns := []string{"/health", "/ping", "/ready", "/__", "/sockjs-node", "/ws"}
		for path, count := range urlCounts {
			if count < 20 {
				continue
			}
			// Check if it looks like infrastructure
			isInfra := false
			for _, pat := range infraPatterns {
				if strings.Contains(path, pat) {
					isInfra = true
					break
				}
			}
			if !isInfra {
				continue
			}

			// Check if already covered
			if nc.isURLCoveredLocked(path) {
				continue
			}

			proposal := NoiseProposal{
				Rule: NoiseRule{
					Category:       "network",
					Classification: "infrastructure",
					AutoDetected:   true,
					MatchSpec: NoiseMatchSpec{
						URLRegex: regexp.QuoteMeta(path),
					},
				},
				Confidence: 0.8,
				Reason:     fmt.Sprintf("infrastructure path hit %d times", count),
			}
			proposals = append(proposals, proposal)
		}
	}

	// Auto-apply high-confidence proposals
	for i := range proposals {
		if proposals[i].Confidence < 0.9 || len(nc.rules) >= maxNoiseRules {
			continue
		}
		nc.userIDCounter++
		rule := proposals[i].Rule
		rule.ID = fmt.Sprintf("auto_%d", nc.userIDCounter)
		rule.CreatedAt = time.Now()
		nc.rules = append(nc.rules, rule)
	}

	nc.recompile()
	return proposals
}

// isConsoleCoveredLocked checks if a message is already covered by existing rules (caller holds lock)
func (nc *NoiseConfig) isConsoleCoveredLocked(msg string, entries []LogEntry) bool {
	for i := range nc.compiled {
		if nc.compiled[i].rule.Category != "console" {
			continue
		}
		if nc.compiled[i].messageRegex != nil && nc.compiled[i].messageRegex.MatchString(msg) {
			return true
		}
	}
	// Also check if the source of those entries is already covered
	for _, entry := range entries {
		entryMsg, _ := entry["message"].(string)
		if entryMsg != msg {
			continue
		}
		source, _ := entry["source"].(string)
		for i := range nc.compiled {
			if nc.compiled[i].rule.Category != "console" {
				continue
			}
			if nc.compiled[i].sourceRegex != nil && nc.compiled[i].sourceRegex.MatchString(source) {
				return true
			}
		}
	}
	return false
}

// isSourceCoveredLocked checks if a source is already covered by existing rules (caller holds lock)
func (nc *NoiseConfig) isSourceCoveredLocked(source string) bool {
	for i := range nc.compiled {
		if nc.compiled[i].rule.Category != "console" {
			continue
		}
		if nc.compiled[i].sourceRegex != nil && nc.compiled[i].sourceRegex.MatchString(source) {
			return true
		}
	}
	return false
}

// isURLCoveredLocked checks if a URL path is already covered by existing rules (caller holds lock)
func (nc *NoiseConfig) isURLCoveredLocked(path string) bool {
	for i := range nc.compiled {
		if nc.compiled[i].rule.Category != "network" {
			continue
		}
		if nc.compiled[i].urlRegex != nil && nc.compiled[i].urlRegex.MatchString(path) {
			return true
		}
	}
	return false
}
