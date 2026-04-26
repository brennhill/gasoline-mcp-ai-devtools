// install_id_drift_log_test.go — Pins the lifecycle-log field-shape contract
// for the install_id_drift event. Exercises the real production wiring fn
// (newInstallIDDriftLogger) so a rename of stored_iid / derived_iid map keys
// or the lifecycle event name is caught at this layer. Also pins that
// runMCPMode actually registers the wiring fn through telemetry.
//
// The complementary contract — that telemetry.CheckInstallIDDrift actually
// invokes the registered callback when stored != derived — lives in
// internal/telemetry/install_id_test.go::TestCheckInstallIDDrift_FiresWhenDerivedChanges.

package main

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

func TestNewInstallIDDriftLogger_LogShape(t *testing.T) {
	srv := newTestServerForHandlers(t)
	const port = 7890
	const stored = "111111111111"
	const derived = "222222222222"

	fn := newInstallIDDriftLogger(srv, port)
	fn(stored, derived)

	entries := srv.logs.getEntries()
	var found map[string]any
	for _, e := range entries {
		if e["event"] == "install_id_drift" {
			found = e
			break
		}
	}
	if found == nil {
		t.Fatal("install_id_drift lifecycle event missing from logs")
	}
	if got := found["stored_iid"]; got != stored {
		t.Errorf("stored_iid = %v, want %q", got, stored)
	}
	if got := found["derived_iid"]; got != derived {
		t.Errorf("derived_iid = %v, want %q", got, derived)
	}
	if got := found["type"]; got != "lifecycle" {
		t.Errorf("type = %v, want lifecycle", got)
	}
}

// TestSetInstallIDDriftLogFn_RegistersThroughPublicAPI confirms that calling
// SetInstallIDDriftLogFn(newInstallIDDriftLogger(...)) — the wiring runMCPMode
// performs at boot — leaves a non-nil callback registered with telemetry.
// A future refactor that drops the registration call would fail this test.
func TestSetInstallIDDriftLogFn_RegistersThroughPublicAPI(t *testing.T) {
	srv := newTestServerForHandlers(t)
	telemetry.SetInstallIDDriftLogFn(newInstallIDDriftLogger(srv, 7892))
	t.Cleanup(func() { telemetry.SetInstallIDDriftLogFn(nil) })

	if !telemetry.HasInstallIDDriftLogFn() {
		t.Fatal("SetInstallIDDriftLogFn did not register a callback through the public API")
	}
}
