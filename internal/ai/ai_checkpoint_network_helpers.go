package ai

import "github.com/dev-console/dev-console/internal/capture"

func classifyNetworkBody(diff *NetworkDiff, body capture.NetworkBody, known map[string]endpointState) {
	path := capture.ExtractURLPath(body.URL)

	if body.Status >= 400 {
		classifyFailedRequest(diff, path, body.Status, known)
		return
	}
	classifySuccessfulRequest(diff, path, body.Duration, known)
}

func classifyFailedRequest(diff *NetworkDiff, path string, status int, known map[string]endpointState) {
	if prev, ok := known[path]; ok && prev.Status < 400 {
		diff.Failures = append(diff.Failures, NetworkFailure{
			Path:           path,
			Status:         status,
			PreviousStatus: prev.Status,
		})
	} else if !ok {
		appendUniqueEndpoint(diff, path)
	}
}

func classifySuccessfulRequest(diff *NetworkDiff, path string, duration int, known map[string]endpointState) {
	if _, ok := known[path]; !ok {
		appendUniqueEndpoint(diff, path)
	}
	if duration <= 0 {
		return
	}
	if prev, ok := known[path]; ok && prev.Duration > 0 && duration > prev.Duration*degradedLatencyFactor {
		diff.Degraded = append(diff.Degraded, NetworkDegraded{
			Path:     path,
			Duration: duration,
			Baseline: prev.Duration,
		})
	}
}

func appendUniqueEndpoint(diff *NetworkDiff, path string) {
	if !containsString(diff.NewEndpoints, path) {
		diff.NewEndpoints = append(diff.NewEndpoints, path)
	}
}

func capNetworkDiff(diff *NetworkDiff) {
	if len(diff.Failures) > maxDiffEntriesPerCat {
		diff.Failures = diff.Failures[:maxDiffEntriesPerCat]
	}
	if len(diff.NewEndpoints) > maxDiffEntriesPerCat {
		diff.NewEndpoints = diff.NewEndpoints[:maxDiffEntriesPerCat]
	}
	if len(diff.Degraded) > maxDiffEntriesPerCat {
		diff.Degraded = diff.Degraded[:maxDiffEntriesPerCat]
	}
}

func (cm *CheckpointManager) buildKnownEndpoints(existing map[string]endpointState) map[string]endpointState {
	result := make(map[string]endpointState)

	for k, v := range existing {
		result[k] = v
	}

	for _, body := range cm.capture.GetNetworkBodies() {
		path := capture.ExtractURLPath(body.URL)
		result[path] = endpointState{
			Status:   body.Status,
			Duration: body.Duration,
		}
	}

	return result
}
