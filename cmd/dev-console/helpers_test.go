package main

import (
	"testing"
)

func setupTestCapture(t *testing.T) *Capture {
	t.Helper()
	return NewCapture()
}
