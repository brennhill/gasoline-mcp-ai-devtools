// server_routes_upgrade.go — HTTP handlers for the extension-triggered self-update flow.
// Docs: docs/features/feature/self-update/index.md

package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upgrade"
)

// upgradeInstallURL is the pinned installer location. The daemon never accepts
// a URL from the caller — this prevents the endpoint from becoming an arbitrary
// command-execution primitive if some other Origin check ever regresses.
const upgradeInstallURL = "https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/install.sh"

// upgradeRateLimitWindow bounds how often the install endpoint can be called.
// One minute is enough to block accidental double-clicks and repeated retries
// during a failed install without being user-hostile.
const upgradeRateLimitWindow = time.Minute

// upgradeSpawnFn is the detached-install launcher. Tests swap this with a fake.
var upgradeSpawnFn = upgrade.Spawn

// upgradeReqBody is the JSON shape accepted by /upgrade/install.
type upgradeReqBody struct {
	Nonce string `json:"nonce"`
}

// handleUpgradeNonce returns the current per-process nonce and pins it to the
// requesting Origin. The extension calls this over its authenticated channel
// before posting /upgrade/install so the install endpoint can reject both
// unauthenticated local callers and a second extension on the same browser
// that would otherwise replay a nonce it harvested over localhost.
func (s *Server) handleUpgradeNonce(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		// Empty Origin defeats the nonce-origin binding. corsMiddleware already
		// accepts empty Origin (CLI/curl paths) but for this endpoint we must
		// reject so the nonce is never issued without a matching pin target.
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Origin header required"})
		return
	}
	s.upgradeNonce.Pin(origin)
	jsonResponse(w, http.StatusOK, map[string]string{"nonce": s.upgradeNonce.Current()})
}

// handleUpgradeInstall launches the pinned installer in a detached process and
// returns immediately. The daemon will be SIGTERM'd by the script once the new
// binary is in place; the supervisor (launchd/systemd) respawns with it.
func (s *Server) handleUpgradeInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	var body upgradeReqBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if !s.upgradeNonce.Verify(body.Nonce, r.Header.Get("Origin")) {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "Invalid nonce"})
		return
	}

	s.upgradeMu.Lock()
	if !s.lastUpgradeAttempt.IsZero() && time.Since(s.lastUpgradeAttempt) < upgradeRateLimitWindow {
		s.upgradeMu.Unlock()
		jsonResponse(w, http.StatusTooManyRequests, map[string]string{"error": "Upgrade attempt rate-limited; try again shortly"})
		return
	}
	s.lastUpgradeAttempt = time.Now()
	s.upgradeMu.Unlock()

	if err := upgradeSpawnFn(upgradeInstallURL); err != nil {
		if errors.Is(err, upgrade.ErrUnsupportedPlatform) {
			jsonResponse(w, http.StatusNotImplemented, map[string]string{
				"error": "Self-update is not supported on this platform — re-run the installer manually.",
			})
			return
		}
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to launch installer"})
		return
	}

	jsonResponse(w, http.StatusAccepted, map[string]string{"status": "installing"})
}
