// version_check_test.go — Unit tests for version check helpers.
package main

import (
	"os"
	"testing"
)

func TestGetEnvOrDefault(t *testing.T) {
	t.Parallel()

	t.Run("returns env value when set", func(t *testing.T) {
		key := "GASOLINE_TEST_ENV_VAR_12345"
		os.Setenv(key, "custom_value")
		defer os.Unsetenv(key)

		got := getEnvOrDefault(key, "default")
		if got != "custom_value" {
			t.Fatalf("got %q, want %q", got, "custom_value")
		}
	})

	t.Run("returns default when unset", func(t *testing.T) {
		got := getEnvOrDefault("GASOLINE_NONEXISTENT_VAR_99999", "fallback")
		if got != "fallback" {
			t.Fatalf("got %q, want %q", got, "fallback")
		}
	})

	t.Run("returns default when empty", func(t *testing.T) {
		key := "GASOLINE_TEST_EMPTY_VAR_12345"
		os.Setenv(key, "")
		defer os.Unsetenv(key)

		got := getEnvOrDefault(key, "default_val")
		if got != "default_val" {
			t.Fatalf("got %q, want %q", got, "default_val")
		}
	})
}
