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

func TestParseDOMPrimitiveParams_ParsesScrollDirection(t *testing.T) {
	params, err := parseDOMPrimitiveParams(json.RawMessage(`{"selector":"#modal","direction":"bottom"}`))
	if err != nil {
		t.Fatalf("parseDOMPrimitiveParams returned error: %v", err)
	}
	if got, want := params.Direction, "bottom"; got != want {
		t.Fatalf("Direction = %q, want %q", got, want)
	}
}

func TestParseDOMPrimitiveParams_ParsesStructuredFlag(t *testing.T) {
	params, err := parseDOMPrimitiveParams(json.RawMessage(`{"selector":".accordion","structured":true}`))
	if err != nil {
		t.Fatalf("parseDOMPrimitiveParams returned error: %v", err)
	}
	if !params.Structured {
		t.Fatal("Structured should be true when provided")
	}
}
