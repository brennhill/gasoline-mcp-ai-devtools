package main

import (
	"encoding/json"
	"testing"
)

func TestParseDOMPrimitiveParams_ParsesNth(t *testing.T) {
	params, err := parseDOMPrimitiveParams(json.RawMessage(`{"selector":"text=Edit & post","nth":-1}`))
	if err != nil {
		t.Fatalf("parseDOMPrimitiveParams returned error: %v", err)
	}
	if params.Nth == nil {
		t.Fatal("Nth should be parsed when provided")
	}
	if got, want := *params.Nth, -1; got != want {
		t.Fatalf("Nth = %d, want %d", got, want)
	}
}

func TestParseDOMPrimitiveParams_RejectsFractionalNth(t *testing.T) {
	_, err := parseDOMPrimitiveParams(json.RawMessage(`{"selector":"text=Edit & post","nth":1.5}`))
	if err == nil {
		t.Fatal("expected fractional nth to be rejected")
	}
}

