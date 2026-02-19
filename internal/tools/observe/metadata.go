// metadata.go — Response metadata helpers for observe tool.
package observe

import (
	"fmt"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/pagination"
)

// ResponseMetadata provides freshness information for buffer-backed observe responses.
type ResponseMetadata struct {
	RetrievedAt string `json:"retrieved_at"`
	IsStale     bool   `json:"is_stale"`
	DataAge     string `json:"data_age"`
}

// BuildResponseMetadata constructs freshness metadata for an observe response.
func BuildResponseMetadata(cap *capture.Capture, newestEntry time.Time) ResponseMetadata {
	now := time.Now()
	meta := ResponseMetadata{
		RetrievedAt: now.Format(time.RFC3339),
		IsStale:     !cap.IsExtensionConnected(),
	}
	if !newestEntry.IsZero() {
		age := now.Sub(newestEntry)
		meta.DataAge = fmt.Sprintf("%.1fs", age.Seconds())
	} else {
		meta.DataAge = "no_data"
	}
	return meta
}

// BuildPaginatedResponseMetadata merges freshness metadata with cursor pagination metadata.
func BuildPaginatedResponseMetadata(cap *capture.Capture, newestEntry time.Time, pMeta *pagination.CursorPaginationMetadata) map[string]any {
	base := BuildResponseMetadata(cap, newestEntry)
	meta := map[string]any{
		"retrieved_at": base.RetrievedAt,
		"is_stale":     base.IsStale,
		"data_age":     base.DataAge,
		"total":        pMeta.Total,
		"has_more":     pMeta.HasMore,
	}
	if pMeta.Cursor != "" {
		meta["cursor"] = pMeta.Cursor
	}
	if pMeta.OldestTimestamp != "" {
		meta["oldest_timestamp"] = pMeta.OldestTimestamp
	}
	if pMeta.NewestTimestamp != "" {
		meta["newest_timestamp"] = pMeta.NewestTimestamp
	}
	if pMeta.CursorRestarted {
		meta["cursor_restarted"] = true
		meta["original_cursor"] = pMeta.OriginalCursor
	}
	if pMeta.Warning != "" {
		meta["warning"] = pMeta.Warning
	}
	return meta
}
