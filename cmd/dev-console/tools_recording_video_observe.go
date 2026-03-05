// Purpose: Lists and filters saved recording metadata for observe queries.
// Why: Keeps read-only metadata scanning separate from upload/reveal HTTP handlers.
// Docs: docs/features/feature/tab-recording/index.md

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// collectRecordingMetadata scans recording directories and returns deduplicated metadata files.
func collectRecordingMetadata(dirs []string) []string {
	matches := make([]string, 0)
	seen := make(map[string]bool)
	for _, dir := range dirs {
		dirMatches, globErr := filepath.Glob(filepath.Join(dir, "*_meta.json")) // nosemgrep: go_filesystem_rule-fileread
		if globErr != nil {
			continue
		}
		for _, m := range dirMatches {
			if seen[m] {
				continue
			}
			seen[m] = true
			matches = append(matches, m)
		}
	}
	return matches
}

// loadAndFilterRecordings reads metadata files, deduplicates by name, and applies URL filter.
func loadAndFilterRecordings(matches []string, urlFilter string) ([]VideoRecordingMetadata, int64) {
	var recordings []VideoRecordingMetadata
	var totalSize int64
	seenByName := make(map[string]bool)

	for _, metaPath := range matches {
		data, err := os.ReadFile(metaPath) // nosemgrep: go_filesystem_rule-fileread -- CLI tool reads local recording metadata
		if err != nil {
			continue
		}
		var meta VideoRecordingMetadata
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}
		if seenByName[meta.Name] {
			continue
		}
		seenByName[meta.Name] = true

		if urlFilter != "" && !recordingMatchesFilter(meta, urlFilter) {
			continue
		}

		recordings = append(recordings, meta)
		totalSize += meta.SizeBytes
	}

	sort.Slice(recordings, func(i, j int) bool {
		return recordings[i].CreatedAt > recordings[j].CreatedAt
	})

	return recordings, totalSize
}

// recordingMatchesFilter checks if a recording's name or URL contains the filter string (case-insensitive).
func recordingMatchesFilter(meta VideoRecordingMetadata, filter string) bool {
	lower := strings.ToLower(filter)
	return strings.Contains(strings.ToLower(meta.Name), lower) ||
		strings.Contains(strings.ToLower(meta.URL), lower)
}

// toolObserveSavedVideos handles observe({what: "saved_videos"}).
// Globs state recordings metadata files and returns recording metadata.
func (h *ToolHandler) toolObserveSavedVideos(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		URL   string `json:"url"`
		LastN int    `json:"last_n,omitempty"`
	}
		if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	dirs := recordingsReadDirs()
	if len(dirs) == 0 {
		return fail(req, ErrInternal, "Could not resolve recordings directory", "Check disk permissions")
	}

	matches := collectRecordingMetadata(dirs)
	if len(matches) == 0 {
		return succeed(req, "No saved videos", map[string]any{
			"recordings":         []any{},
			"total":              0,
			"storage_used_bytes": int64(0),
		})
	}

	recordings, totalSize := loadAndFilterRecordings(matches, params.URL)

	if params.LastN > 0 && len(recordings) > params.LastN {
		recordings = recordings[:params.LastN]
	}

	return succeed(req, fmt.Sprintf("%d saved videos", len(recordings)), map[string]any{
		"recordings":         recordings,
		"total":              len(recordings),
		"storage_used_bytes": totalSize,
	})
}
