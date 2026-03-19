// scaffold_wizard.go — Serves the scaffold wizard landing page and registers scaffold routes.

package main

import (
	_ "embed"
	"net/http"
)

//go:embed scaffold_wizard.html
var scaffoldWizardHTML []byte

// registerScaffoldRoutes adds scaffold wizard routes to the mux.
func registerScaffoldRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/launch", handleWizardLaunch)
}

// handleWizardLaunch serves the scaffold wizard landing page.
func handleWizardLaunch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	serveEmbeddedHTML(w, r, scaffoldWizardHTML, "wizard")
}
