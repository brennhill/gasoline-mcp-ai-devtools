// network-diff.go — Network diff computation.
// diffNetwork function compares network requests between two snapshots.
package session

import (
	"fmt"

	"github.com/dev-console/dev-console/internal/capture"
)

// endpointKey uniquely identifies a network endpoint by method and path.
type endpointKey struct {
	Method string
	Path   string
}

// buildEndpointMap indexes network requests by (method, path).
func buildEndpointMap(requests []SnapshotNetworkRequest) map[endpointKey]SnapshotNetworkRequest {
	m := make(map[endpointKey]SnapshotNetworkRequest, len(requests))
	for _, req := range requests {
		key := endpointKey{Method: req.Method, Path: capture.ExtractURLPath(req.URL)}
		m[key] = req
	}
	return m
}

// formatDurationChange returns a formatted duration delta string, or "" if not applicable.
func formatDurationChange(beforeDur, afterDur int) string {
	if beforeDur <= 0 || afterDur <= 0 {
		return ""
	}
	delta := afterDur - beforeDur
	if delta >= 0 {
		return fmt.Sprintf("+%dms", delta)
	}
	return fmt.Sprintf("%dms", delta)
}

// diffNetwork compares network requests between two snapshots.
// Requests are matched by (method, URL path) — query params are ignored.
func (sm *SessionManager) diffNetwork(a, b *NamedSnapshot) SessionNetworkDiff {
	diff := SessionNetworkDiff{
		NewErrors:        make([]SnapshotNetworkRequest, 0),
		StatusChanges:    make([]SessionNetworkChange, 0),
		NewEndpoints:     make([]SnapshotNetworkRequest, 0),
		MissingEndpoints: make([]SnapshotNetworkRequest, 0),
	}

	aEndpoints := buildEndpointMap(a.NetworkRequests)
	bEndpoints := buildEndpointMap(b.NetworkRequests)

	// New endpoints = in B but not in A
	for key, req := range bEndpoints {
		if _, found := aEndpoints[key]; !found {
			diff.NewEndpoints = append(diff.NewEndpoints, req)
			if req.Status >= 400 {
				diff.NewErrors = append(diff.NewErrors, req)
			}
		}
	}

	// Missing endpoints = in A but not in B
	for key, req := range aEndpoints {
		if _, found := bEndpoints[key]; !found {
			diff.MissingEndpoints = append(diff.MissingEndpoints, req)
		}
	}

	// Status changes = same endpoint, different status
	for key, aReq := range aEndpoints {
		bReq, found := bEndpoints[key]
		if !found || aReq.Status == bReq.Status {
			continue
		}
		change := SessionNetworkChange{
			Method:         key.Method,
			URL:            aReq.URL,
			BeforeStatus:   aReq.Status,
			AfterStatus:    bReq.Status,
			DurationChange: formatDurationChange(aReq.Duration, bReq.Duration),
		}
		diff.StatusChanges = append(diff.StatusChanges, change)
		if bReq.Status >= 400 && aReq.Status < 400 {
			diff.NewErrors = append(diff.NewErrors, bReq)
		}
	}

	return diff
}
