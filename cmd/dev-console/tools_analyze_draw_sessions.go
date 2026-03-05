// Purpose: Handles draw session history/file retrieval and store hydration.
// Why: Isolates draw-session persistence concerns from annotation retrieval handlers.
// Docs: docs/features/feature/annotated-screenshots/index.md

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// toolListDrawHistory lists all persisted draw session files from disk.
func (h *ToolHandler) toolListDrawHistory(req JSONRPCRequest, _ json.RawMessage) JSONRPCResponse {
	dir, err := screenshotsDir()
	if err != nil {
		return fail(req, ErrNoData, "Cannot access screenshots directory: "+err.Error(), "Check file permissions")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fail(req, ErrNoData, "Cannot read screenshots directory: "+err.Error(), "Check file permissions")
	}

	type sessionSummary struct {
		File      string `json:"file"`
		Path      string `json:"path"`
		SizeBytes int64  `json:"size_bytes"`
		ModTime   int64  `json:"mod_time"`
	}

	sessions := make([]sessionSummary, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "draw-session-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		sessions = append(sessions, sessionSummary{
			File:      entry.Name(),
			Path:      filepath.Join(dir, entry.Name()),
			SizeBytes: info.Size(),
			ModTime:   info.ModTime().UnixMilli(),
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModTime > sessions[j].ModTime
	})

	return succeed(req, "Draw session history", map[string]any{
		"sessions":    sessions,
		"count":       len(sessions),
		"storage_dir": dir,
	})
}

// toolGetDrawSession reads a specific draw session file from disk.
func (h *ToolHandler) toolGetDrawSession(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		File string `json:"file"`
	}
	if len(args) > 0 {
				if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	if params.File == "" {
		return fail(req, ErrMissingParam, "Required parameter 'file' is missing", "Provide the session filename from draw_history results", withParam("file"))
	}

	// Validate filename to prevent path traversal.
	if strings.Contains(params.File, "/") || strings.Contains(params.File, "\\") || strings.Contains(params.File, "..") {
		return fail(req, ErrInvalidParam, "Invalid filename: path traversal not allowed", "Use only the filename from draw_history results", withParam("file"))
	}

	dir, err := screenshotsDir()
	if err != nil {
		return fail(req, ErrNoData, "Cannot access screenshots directory: "+err.Error(), "Check file permissions")
	}

	path := filepath.Join(dir, params.File)
	if !isWithinDir(path, dir) {
		return fail(req, ErrInvalidParam, "Invalid filename: resolved path outside screenshots directory", "Use only the filename from draw_history results", withParam("file"))
	}
	data, err := os.ReadFile(path) // #nosec G304 -- filename validated against path traversal above
	if err != nil {
		if os.IsNotExist(err) {
			return fail(req, ErrNoData, "Draw session file not found: "+params.File, "Use analyze({what:'draw_history'}) to list available sessions")
		}
		return fail(req, ErrNoData, "Cannot read draw session file: "+err.Error(), "Check file permissions")
	}

	var session map[string]any
	if err := json.Unmarshal(data, &session); err != nil {
		return fail(req, ErrInvalidJSON, "Corrupted draw session file: "+err.Error(), "The file may be damaged. Try a different session.")
	}

	// Hydrate in-memory annotation stores so generate.annotation_* can consume
	// sessions loaded from disk by analyze.draw_session.
	h.hydrateAnnotationStoreFromDrawSession(data)

	if name, ok := session["annot_session_name"].(string); ok && strings.TrimSpace(name) != "" {
		// Alias for generate tool parameter naming.
		session["annot_session"] = name
	}
	session["_file"] = params.File
	session["_path"] = path

	return succeed(req, "Draw session loaded", session)
}

type persistedDrawSession struct {
	Annotations      []Annotation               `json:"annotations"`
	ElementDetails   map[string]json.RawMessage `json:"element_details"`
	PageURL          string                     `json:"page_url"`
	TabID            int                        `json:"tab_id"`
	Screenshot       string                     `json:"screenshot"`
	Timestamp        int64                      `json:"timestamp"`
	AnnotSessionName string                     `json:"annot_session_name"`
}

func (h *ToolHandler) hydrateAnnotationStoreFromDrawSession(raw []byte) {
	var persisted persistedDrawSession
	if err := json.Unmarshal(raw, &persisted); err != nil {
		return
	}

	if persisted.TabID > 0 {
		session := &AnnotationSession{
			Annotations:    persisted.Annotations,
			ScreenshotPath: persisted.Screenshot,
			PageURL:        persisted.PageURL,
			TabID:          persisted.TabID,
			Timestamp:      persisted.Timestamp,
		}
		if session.Timestamp == 0 {
			session.Timestamp = time.Now().UnixMilli()
		}
		h.annotationStore.StoreSession(session.TabID, session)

		name := strings.TrimSpace(persisted.AnnotSessionName)
		if name != "" && !namedSessionHasPage(h.annotationStore, name, session) {
			h.annotationStore.AppendToNamedSession(name, session)
		}
	}

	for correlationID, rawDetail := range persisted.ElementDetails {
		var detail AnnotationDetail
		if err := json.Unmarshal(rawDetail, &detail); err != nil {
			continue
		}
		detail.CorrelationID = correlationID
		h.annotationStore.StoreDetail(correlationID, detail)
	}
}

func namedSessionHasPage(store *AnnotationStore, name string, session *AnnotationSession) bool {
	ns := store.GetNamedSession(name)
	if ns == nil {
		return false
	}
	for _, page := range ns.Pages {
		if page.TabID == session.TabID &&
			page.Timestamp == session.Timestamp &&
			page.PageURL == session.PageURL {
			return true
		}
	}
	return false
}
