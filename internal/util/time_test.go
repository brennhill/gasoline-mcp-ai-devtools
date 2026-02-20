// time_test.go — Tests for timestamp parsing utilities.
package util

import (
	"testing"
	"time"
)

func TestParseTimestamp_RFC3339(t *testing.T) {
	t.Parallel()
	got := ParseTimestamp("2024-01-15T10:30:00Z")
	want := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ParseTimestamp(RFC3339) = %v, want %v", got, want)
	}
}

func TestParseTimestamp_RFC3339Nano(t *testing.T) {
	t.Parallel()
	got := ParseTimestamp("2024-01-15T10:30:00.123456789Z")
	want := time.Date(2024, 1, 15, 10, 30, 0, 123456789, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ParseTimestamp(RFC3339Nano) = %v, want %v", got, want)
	}
}

func TestParseTimestamp_RFC3339WithOffset(t *testing.T) {
	t.Parallel()
	got := ParseTimestamp("2024-01-15T10:30:00+05:00")
	if got.IsZero() {
		t.Error("ParseTimestamp(RFC3339 with offset) returned zero time")
	}
}

func TestParseTimestamp_EmptyString(t *testing.T) {
	t.Parallel()
	got := ParseTimestamp("")
	if !got.IsZero() {
		t.Errorf("ParseTimestamp(empty) = %v, want zero time", got)
	}
}

func TestParseTimestamp_InvalidString(t *testing.T) {
	t.Parallel()
	got := ParseTimestamp("not-a-timestamp")
	if !got.IsZero() {
		t.Errorf("ParseTimestamp(invalid) = %v, want zero time", got)
	}
}

func TestParseTimestamp_RFC3339NanoMilliseconds(t *testing.T) {
	t.Parallel()
	got := ParseTimestamp("2024-06-15T08:00:00.500Z")
	if got.IsZero() {
		t.Error("ParseTimestamp(RFC3339Nano millis) returned zero time")
	}
	if got.Nanosecond() != 500000000 {
		t.Errorf("ParseTimestamp nanosecond = %d, want 500000000", got.Nanosecond())
	}
}
