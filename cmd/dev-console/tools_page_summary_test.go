package main

import (
	"strings"
	"testing"
)

func TestSummaryScriptSubstitution(t *testing.T) {
	// This test ensures the Go server correctly prepares the script for the extension
	// by substituting the 'mode' parameter.

	compact := compactSummaryScript()
	if !strings.Contains(compact, "('compact')") {
		t.Errorf("compactSummaryScript() should end with ('compact'), got: %s", compact)
	}

	full := fullSummaryScript()
	if !strings.Contains(full, "('full')") {
		t.Errorf("fullSummaryScript() should end with ('full'), got: %s", full)
	}
}
