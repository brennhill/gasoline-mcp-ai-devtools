// network-diff.go — Network diff computation.
// diffNetwork function compares network requests between two snapshots.
package session

import (
	"fmt"

	"github.com/dev-console/dev-console/internal/capture"
)

// diffNetwork compares network requests between two snapshots.
// Requests are matched by (method, URL path) — query params are ignored.
func (sm *SessionManager) diffNetwork(a, b *NamedSnapshot) SessionNetworkDiff {
	diff := SessionNetworkDiff{
		NewErrors:        make([]SnapshotNetworkRequest, 0),
		StatusChanges:    make([]SessionNetworkChange, 0),
		NewEndpoints:     make([]SnapshotNetworkRequest, 0),
		MissingEndpoints: make([]SnapshotNetworkRequest, 0),
	}

	type endpointKey struct {
		Method string
		Path   string
	}

	// Build maps by (method, path)
	aEndpoints := make(map[endpointKey]SnapshotNetworkRequest)
	for _, req := range a.NetworkRequests {
		key := endpointKey{Method: req.Method, Path: capture.ExtractURLPath(req.URL)}
		aEndpoints[key] = req
	}

	bEndpoints := make(map[endpointKey]SnapshotNetworkRequest)
	for _, req := range b.NetworkRequests {
		key := endpointKey{Method: req.Method, Path: capture.ExtractURLPath(req.URL)}
		bEndpoints[key] = req
	}

	// New endpoints = in B but not in A
	for key, req := range bEndpoints {
		if _, found := aEndpoints[key]; !found {
			diff.NewEndpoints = append(diff.NewEndpoints, req)
			// If new endpoint is an error (4xx/5xx), also add to new_errors
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
		if bReq, found := bEndpoints[key]; found {
			if aReq.Status != bReq.Status {
				change := SessionNetworkChange{
					Method:       key.Method,
					URL:          aReq.URL,
					BeforeStatus: aReq.Status,
					AfterStatus:  bReq.Status,
				}
				// Compute duration change if both have duration
				if aReq.Duration > 0 && bReq.Duration > 0 {
					delta := bReq.Duration - aReq.Duration
					if delta >= 0 {
						change.DurationChange = fmt.Sprintf("+%dms", delta)
					} else {
						change.DurationChange = fmt.Sprintf("%dms", delta)
					}
				}
				diff.StatusChanges = append(diff.StatusChanges, change)
				// A status change to 4xx/5xx is also a new error
				if bReq.Status >= 400 && aReq.Status < 400 {
					diff.NewErrors = append(diff.NewErrors, bReq)
				}
			}
		}
	}

	return diff
}
