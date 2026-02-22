package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/dev-console/dev-console/internal/capture"
)

func TestInsecureProxyEndpoint_StripsCSPHeaders(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		w.Header().Set("Content-Security-Policy-Report-Only", "default-src 'none'")
		w.Header().Set("X-Frame-Options", "DENY")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>fixture</body></html>"))
	}))
	defer upstream.Close()

	srv := newTestServerForHandlers(t)
	cap := capture.NewCapture()
	cap.SetSecurityMode("insecure_proxy", []string{"csp_headers"})
	mux := setupHTTPRoutes(srv, cap)

	req := localRequest(http.MethodGet, "/insecure-proxy?target="+url.QueryEscape(upstream.URL), nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /insecure-proxy status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Content-Security-Policy"); got != "" {
		t.Fatalf("CSP header should be stripped, got %q", got)
	}
	if got := rr.Header().Get("Content-Security-Policy-Report-Only"); got != "" {
		t.Fatalf("CSP report-only header should be stripped, got %q", got)
	}
	if got := rr.Header().Get("X-Gasoline-Proxy-Mode"); got != "insecure_proxy" {
		t.Fatalf("X-Gasoline-Proxy-Mode = %q, want insecure_proxy", got)
	}
	body, _ := io.ReadAll(rr.Body)
	if !strings.Contains(string(body), "fixture") {
		t.Fatalf("proxy body should include upstream content, got: %s", string(body))
	}
}

func TestInsecureProxyEndpoint_RequiresInsecureMode(t *testing.T) {
	t.Parallel()
	srv := newTestServerForHandlers(t)
	cap := capture.NewCapture()
	cap.SetSecurityMode("normal", nil)
	mux := setupHTTPRoutes(srv, cap)

	req := localRequest(http.MethodGet, "/insecure-proxy?target="+url.QueryEscape("https://example.com"), nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("GET /insecure-proxy status = %d, want 403 when mode is normal", rr.Code)
	}
}
