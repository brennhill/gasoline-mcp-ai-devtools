package main

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
)

var insecureProxyStripHeaders = map[string]bool{
	"content-security-policy":             true,
	"content-security-policy-report-only": true,
	"x-content-security-policy":           true,
	"x-webkit-csp":                        true,
}

func (s *Server) handleInsecureProxy(w http.ResponseWriter, r *http.Request, cap *capture.Capture) {
	if r.Method != http.MethodGet {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	if cap == nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Capture unavailable"})
		return
	}

	mode, productionParity, rewrites := cap.GetSecurityMode()
	if mode != capture.SecurityModeInsecureProxy {
		jsonResponse(w, http.StatusForbidden, map[string]string{
			"error": "insecure proxy is disabled; enable configure(what='security_mode', mode='insecure_proxy', confirm=true)",
		})
		return
	}

	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if target == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing target query parameter"})
		return
	}
	targetURL, err := url.Parse(target)
	if err != nil || targetURL.Host == "" || (targetURL.Scheme != "http" && targetURL.Scheme != "https") {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid target URL"})
		return
	}

	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
	if err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid target URL"})
		return
	}
	if accept := r.Header.Get("Accept"); accept != "" {
		upstreamReq.Header.Set("Accept", accept)
	}
	if ua := r.Header.Get("User-Agent"); ua != "" {
		upstreamReq.Header.Set("User-Agent", ua)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	upstreamResp, err := client.Do(upstreamReq)
	if err != nil {
		jsonResponse(w, http.StatusBadGateway, map[string]string{"error": "Failed to fetch target URL"})
		return
	}
	defer upstreamResp.Body.Close() //nolint:errcheck

	for key, values := range upstreamResp.Header {
		if insecureProxyStripHeaders[strings.ToLower(key)] {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.Header().Set("X-Gasoline-Proxy-Mode", mode)
	w.Header().Set("X-Gasoline-Production-Parity", "false")
	if len(rewrites) > 0 {
		w.Header().Set("X-Gasoline-Insecure-Rewrites", strings.Join(rewrites, ","))
	}
	if productionParity {
		w.Header().Set("X-Gasoline-Production-Parity", "true")
	}
	w.WriteHeader(upstreamResp.StatusCode)
	_, _ = io.Copy(w, upstreamResp.Body)
}
