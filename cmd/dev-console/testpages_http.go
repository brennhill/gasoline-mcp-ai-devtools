// Purpose: Serves embedded test/demo pages and deterministic HTTP fixtures under /tests/.
// Why: Provides self-contained smoke-test HTTP fixtures (404, 500, CORS, slow) without external dependencies.
// Docs: docs/features/feature/self-testing/index.md

package main

import (
	"embed"
	"fmt"
	"html"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed testpages
var testPagesFS embed.FS

// handleTestPages serves embedded test pages at /tests/.
// GET /tests/          → branded HTML index listing all pages.
// GET /tests/{file}    → embedded HTML/CSS file.
// GET /tests/404       → 404 response (network error test).
// GET /tests/500       → 500 response (network error test).
// GET /tests/cors-test → 200 JSON, no CORS headers (CORS-block test).
// GET /tests/slow      → 200 after 3 s delay (latency/waterfall test).
func handleTestPages() http.HandlerFunc {
	sub, err := fs.Sub(testPagesFS, "testpages")
	if err != nil {
		panic(fmt.Sprintf("testpages: embed misconfigured: %v", err))
	}
	fileServer := http.StripPrefix("/tests/", http.FileServerFS(sub))

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}

		trimmed := strings.TrimPrefix(r.URL.Path, "/tests")
		trimmed = strings.TrimPrefix(trimmed, "/")

		switch trimmed {
		case "":
			serveTestIndex(w)
		case "404":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprintln(w, `{"error":"not_found","endpoint":"/tests/404","test":"network_error"}`)
		case "500":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintln(w, `{"error":"internal_server_error","endpoint":"/tests/500","test":"network_error"}`)
		case "cors-test":
			// Intentionally omit CORS headers — browser blocks cross-origin fetch.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintln(w, `{"cors":"no_headers","test":"cors_block"}`)
		case "slow":
			time.Sleep(3 * time.Second)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintln(w, `{"delayed":true,"delay_ms":3000,"test":"slow_response"}`)
		default:
			fileServer.ServeHTTP(w, r)
		}
	}
}

// serveTestIndex generates a branded HTML index of available test pages.
// File names and labels are HTML-escaped to prevent injection from any
// unexpected entries in the embedded filesystem.
func serveTestIndex(w http.ResponseWriter) {
	entries, err := fs.ReadDir(testPagesFS, "testpages")
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{
			"error":   "internal_error",
			"message": "failed to read test pages",
		})
		return
	}

	var links []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".html") {
			continue
		}
		safeName := html.EscapeString(name)
		safeLabel := html.EscapeString(strings.TrimSuffix(name, path.Ext(name)))
		links = append(links, fmt.Sprintf(`<li><a href="/tests/%s">%s</a></li>`, safeName, safeLabel))
	}

	body := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>Gasoline Test Pages</title>
<style>
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;
     background:#1a1a1a;color:#e0e0e0;padding:40px;max-width:600px;margin:0 auto}
h1{font-size:20px;color:#fff;margin-bottom:4px}
p{font-size:13px;color:#888;margin-bottom:24px}
ul{list-style:none;padding:0}
li{margin:6px 0}
a{color:#58a6ff;text-decoration:none;font-size:14px}
a:hover{text-decoration:underline}
.ep{font-family:monospace;font-size:12px;color:#888;margin-top:20px;padding-top:16px;border-top:1px solid #333}
.ep a{color:#d29922}
</style></head>
<body>
<h1>🔥 Gasoline — Test Harness</h1>
<p>Deterministic smoke-test pages served by the Gasoline Go daemon.</p>
<ul>%s</ul>
<div class="ep">
  <div>Test endpoints:</div>
  <div><a href="/tests/404">/tests/404</a> — 404 Not Found</div>
  <div><a href="/tests/500">/tests/500</a> — 500 Server Error</div>
  <div><a href="/tests/cors-test">/tests/cors-test</a> — no CORS headers</div>
  <div><a href="/tests/slow">/tests/slow</a> — 3 s delay</div>
  <div>ws://{host}/tests/ws — WebSocket echo</div>
</div>
</body></html>`, strings.Join(links, "\n"))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, body)
}
