package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	statecfg "github.com/dev-console/dev-console/internal/state"
)

func TestHandleHealthIncludesBridgeFastPathCounters(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	resetFastPathResourceReadCounters()
	recordFastPathResourceRead("gasoline://capabilities", true, 0)
	recordFastPathResourceRead("gasoline://playbook/nonexistent/quick", false, -32002)

	s := &Server{
		maxEntries: 100,
		logFile:    filepath.Join(t.TempDir(), "gasoline.jsonl"),
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	s.handleHealth(rr, req, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("handleHealth status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal(health) error = %v", err)
	}
	fastPath, ok := body["bridge_fastpath"].(map[string]any)
	if !ok {
		t.Fatalf("bridge_fastpath missing or wrong type: %#v", body["bridge_fastpath"])
	}
	if got, _ := fastPath["resources_read_success"].(float64); int(got) != 1 {
		t.Fatalf("resources_read_success = %v, want 1", fastPath["resources_read_success"])
	}
	if got, _ := fastPath["resources_read_failure"].(float64); int(got) != 1 {
		t.Fatalf("resources_read_failure = %v, want 1", fastPath["resources_read_failure"])
	}
}
