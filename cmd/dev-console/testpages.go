// testpages.go — Embedded test/demo pages served at /tests/.
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed testpages
var testPagesFS embed.FS

// handleTestPages serves embedded test pages at /tests/.
// GET /tests/ returns an HTML index listing available pages.
// GET /tests/{filename} serves the embedded file.
func handleTestPages() http.HandlerFunc {
	sub, _ := fs.Sub(testPagesFS, "testpages")
	fileServer := http.StripPrefix("/tests/", http.FileServerFS(sub))

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}

		trimmed := strings.TrimPrefix(r.URL.Path, "/tests")
		trimmed = strings.TrimPrefix(trimmed, "/")

		if trimmed == "" {
			serveTestIndex(w)
			return
		}

		fileServer.ServeHTTP(w, r)
	}
}

// serveTestIndex generates an HTML index of available test pages.
func serveTestIndex(w http.ResponseWriter) {
	entries, err := fs.ReadDir(testPagesFS, "testpages")
	if err != nil {
		http.Error(w, "failed to read test pages", http.StatusInternalServerError)
		return
	}

	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html><head><meta charset=\"UTF-8\">")
	b.WriteString("<title>Test Pages</title>")
	b.WriteString("<style>body{font-family:system-ui,sans-serif;padding:40px;max-width:600px;margin:0 auto}")
	b.WriteString("a{display:block;padding:8px 0;font-size:16px}</style></head><body>")
	b.WriteString("<h1>Test Pages</h1><ul>")

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".html") {
			continue
		}
		label := strings.TrimSuffix(name, path.Ext(name))
		b.WriteString(fmt.Sprintf("<li><a href=\"/tests/%s\">%s</a></li>", name, label))
	}

	b.WriteString("</ul></body></html>")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(b.String()))
}
