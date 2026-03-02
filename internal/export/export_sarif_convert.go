package export

import "strings"

// convertViolationsToResults converts axe violations to SARIF results.
func convertViolationsToResults(run *SARIFRun, ruleIndices map[string]int, violations []axeViolation) {
	for i := range violations {
		v := violations[i]
		ruleIdx := ensureRule(run, ruleIndices, v)
		for _, node := range v.Nodes {
			run.Results = append(run.Results, nodeToResult(v, node, ruleIdx, axeImpactToLevel(node.Impact)))
		}
	}
}

// convertPassesToResults converts axe passes to SARIF results with "none" level.
func convertPassesToResults(run *SARIFRun, ruleIndices map[string]int, passes []axeViolation) {
	for i := range passes {
		p := passes[i]
		ruleIdx := ensureRule(run, ruleIndices, p)
		for _, node := range p.Nodes {
			run.Results = append(run.Results, nodeToResult(p, node, ruleIdx, "none"))
		}
	}
}

// ensureRule adds a rule to the driver rules if not already present, returns the index.
func ensureRule(run *SARIFRun, indices map[string]int, v axeViolation) int {
	if idx, exists := indices[v.ID]; exists {
		return idx
	}

	rule := SARIFRule{
		ID:               v.ID,
		ShortDescription: SARIFMessage{Text: v.Description},
		FullDescription:  SARIFMessage{Text: v.Help},
		HelpURI:          v.HelpURL,
	}

	wcagTags := extractWCAGTags(v.Tags)
	if len(wcagTags) > 0 {
		rule.Properties = &SARIFRuleProperties{Tags: wcagTags}
	}

	idx := len(run.Tool.Driver.Rules)
	run.Tool.Driver.Rules = append(run.Tool.Driver.Rules, rule)
	indices[v.ID] = idx
	return idx
}

// nodeToResult converts a single axe node to a SARIF result.
func nodeToResult(v axeViolation, node axeNode, ruleIndex int, level string) SARIFResult {
	selector := ""
	if len(node.Target) > 0 {
		selector = node.Target[0]
	}

	return SARIFResult{
		RuleID:    v.ID,
		RuleIndex: ruleIndex,
		Level:     level,
		Message:   SARIFMessage{Text: v.Help},
		Locations: []SARIFLocation{{
			PhysicalLocation: SARIFPhysicalLocation{
				ArtifactLocation: SARIFArtifactLocation{
					URI: selector,
				},
				Region: SARIFRegion{
					Snippet: SARIFSnippet{Text: node.HTML},
				},
			},
		}},
	}
}

// axeImpactToLevel maps axe-core impact levels to SARIF levels.
func axeImpactToLevel(impact string) string {
	switch impact {
	case "critical", "serious":
		return "error"
	case "moderate":
		return "warning"
	case "minor":
		return "note"
	default:
		return "warning"
	}
}

// extractWCAGTags filters a slice of axe-core tags to only those starting with "wcag".
func extractWCAGTags(tags []string) []string {
	result := make([]string, 0)
	for _, tag := range tags {
		if strings.HasPrefix(tag, "wcag") {
			result = append(result, tag)
		}
	}
	return result
}
