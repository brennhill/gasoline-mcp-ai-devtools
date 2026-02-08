// actions-diff.go — Actions diff computation.
// diffErrors function compares errors between two snapshots.
package session

// diffErrors computes the set difference of errors between two snapshots.
func (sm *SessionManager) diffErrors(a, b *NamedSnapshot) ErrorDiff {
	diff := ErrorDiff{
		New:       make([]SnapshotError, 0),
		Resolved:  make([]SnapshotError, 0),
		Unchanged: make([]SnapshotError, 0),
	}

	aMessages := make(map[string]SnapshotError)
	for _, e := range a.ConsoleErrors {
		aMessages[e.Message] = e
	}

	bMessages := make(map[string]SnapshotError)
	for _, e := range b.ConsoleErrors {
		bMessages[e.Message] = e
	}

	// New = in B but not in A
	for msg, e := range bMessages {
		if _, found := aMessages[msg]; !found {
			diff.New = append(diff.New, e)
		}
	}

	// Resolved = in A but not in B
	for msg, e := range aMessages {
		if _, found := bMessages[msg]; !found {
			diff.Resolved = append(diff.Resolved, e)
		} else {
			diff.Unchanged = append(diff.Unchanged, e)
		}
	}

	return diff
}

// computeSummary derives the verdict and aggregate counts from diff.
func (sm *SessionManager) computeSummary(result *SessionDiffResult) DiffSummary {
	summary := DiffSummary{
		NewErrors:        len(result.Errors.New),
		ResolvedErrors:   len(result.Errors.Resolved),
		NewNetworkErrors: len(result.Network.NewErrors),
	}

	// Count performance regressions
	if result.Performance.LoadTime != nil && result.Performance.LoadTime.Regression {
		summary.PerformanceRegressions++
	}
	if result.Performance.RequestCount != nil && result.Performance.RequestCount.Regression {
		summary.PerformanceRegressions++
	}
	if result.Performance.TransferSize != nil && result.Performance.TransferSize.Regression {
		summary.PerformanceRegressions++
	}

	// Verdict logic:
	// "improved" if resolved > 0 AND new == 0 AND no regressions
	// "regressed" if new > 0 OR performance_regressions > 0 OR new_network_errors > 0
	// "unchanged" if no differences
	// "mixed" if both resolved and new

	hasRegressions := summary.NewErrors > 0 || summary.PerformanceRegressions > 0 || summary.NewNetworkErrors > 0
	hasImprovements := summary.ResolvedErrors > 0

	// Also check for status changes where a previously-OK endpoint now errors
	for _, sc := range result.Network.StatusChanges {
		if sc.AfterStatus >= 400 && sc.BeforeStatus < 400 {
			hasRegressions = true
			break
		}
	}

	switch {
	case hasRegressions && hasImprovements:
		summary.Verdict = "mixed"
	case hasRegressions:
		summary.Verdict = "regressed"
	case hasImprovements:
		summary.Verdict = "improved"
	default:
		summary.Verdict = "unchanged"
	}

	return summary
}
