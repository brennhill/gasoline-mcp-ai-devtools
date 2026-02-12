// main_unit_test.go — Unit tests for pure functions in main.go.
package main

import (
	"testing"
)

func TestMultiFlag_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		flags multiFlag
		want  string
	}{
		{"empty", multiFlag{}, ""},
		{"single", multiFlag{"foo"}, "foo"},
		{"multiple", multiFlag{"a", "b", "c"}, "a, b, c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.flags.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMultiFlag_Set(t *testing.T) {
	t.Parallel()

	var f multiFlag

	if err := f.Set("first"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if len(f) != 1 || f[0] != "first" {
		t.Fatalf("after Set(first): got %v", f)
	}

	if err := f.Set("second"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if len(f) != 2 || f[1] != "second" {
		t.Fatalf("after Set(second): got %v", f)
	}
}

func TestIsProcessAlive(t *testing.T) {
	t.Parallel()

	t.Run("zero pid", func(t *testing.T) {
		if isProcessAlive(0) {
			t.Fatal("pid=0 should not be alive")
		}
	})

	t.Run("negative pid", func(t *testing.T) {
		if isProcessAlive(-1) {
			t.Fatal("pid=-1 should not be alive")
		}
	})

	t.Run("nonexistent pid", func(t *testing.T) {
		// PID 999999 is almost certainly not running
		if isProcessAlive(999999) {
			t.Skip("PID 999999 is somehow alive; skipping")
		}
	})
}

func TestPidFilePath(t *testing.T) {
	t.Parallel()

	path := pidFilePath(3000)
	if path == "" {
		t.Fatal("pidFilePath(3000) should return a non-empty path")
	}
}

func TestLegacyPIDFilePath(t *testing.T) {
	t.Parallel()

	path := legacyPIDFilePath(3000)
	if path == "" {
		t.Fatal("legacyPIDFilePath(3000) should return a non-empty path")
	}
}
