// scaffold_wizard_test.go — Tests for the scaffold wizard landing page and API endpoint.

package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ============================================
// Wizard Landing Page: GET /launch
// ============================================

func TestWizardLaunch_GET_Returns200WithHTML(t *testing.T) {
	mux := http.NewServeMux()
	registerScaffoldRoutes(mux)

	req := httptest.NewRequest("GET", "/launch", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /launch: want status 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("GET /launch: want Content-Type text/html, got %q", ct)
	}
}

func TestWizardLaunch_GET_ContainsExpectedStructure(t *testing.T) {
	mux := http.NewServeMux()
	registerScaffoldRoutes(mux)

	req := httptest.NewRequest("GET", "/launch", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()

	checks := []struct {
		label    string
		contains string
	}{
		{"html tag", "<html"},
		{"wizard container", `id="wizard"`},
		{"step 1 prompt", "What are you building"},
		{"step 2 prompt", "Who is it for"},
		{"step 3 prompt", "most important first feature"},
		{"step 4 prompt", "Project name"},
		{"create button", "Create"},
		{"progress bar", `id="progress"`},
	}

	for _, c := range checks {
		if !strings.Contains(body, c.contains) {
			t.Errorf("GET /launch: body missing %s (expected %q)", c.label, c.contains)
		}
	}
}

func TestWizardLaunch_POST_MethodNotAllowed(t *testing.T) {
	mux := http.NewServeMux()
	registerScaffoldRoutes(mux)

	req := httptest.NewRequest("POST", "/launch", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /launch: want status 405, got %d", rec.Code)
	}
}
