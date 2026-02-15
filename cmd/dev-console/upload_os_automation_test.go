// upload_os_automation_test.go — Tests for Chrome PID auto-detection and enhanced error messages.
//
// WARNING: DO NOT use t.Parallel() — tests share global state (skipSSRFCheck, uploadSecurityConfig).
//
// Run: go test ./cmd/dev-console -run "TestOSAutomation" -v
package main

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ============================================
// detectBrowserPID tests
// ============================================

func TestOSAutomation_DetectBrowserPID_ReturnsValidPID(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-only integration test")
	}

	pid, err := detectBrowserPID()
	if err != nil {
		t.Skipf("Chrome not running: %v", err)
	}

	if pid <= 0 {
		t.Errorf("detectBrowserPID() returned invalid PID: %d", pid)
	}
}

func TestOSAutomation_DetectBrowserPID_ErrorContainsPgrep_macOS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-only test")
	}

	// We can't force Chrome to not be running, so we test the error message format
	// by examining the function's error path. If Chrome IS running, this test
	// verifies PID is valid instead.
	pid, err := detectBrowserPID()
	if err != nil {
		if !strings.Contains(err.Error(), "pgrep") {
			t.Errorf("macOS error should mention pgrep, got: %s", err.Error())
		}
		if !strings.Contains(err.Error(), "Google Chrome") {
			t.Errorf("macOS error should mention 'Google Chrome', got: %s", err.Error())
		}
	} else if pid <= 0 {
		t.Errorf("no error but invalid PID: %d", pid)
	}
}

func TestOSAutomation_DetectBrowserPID_ErrorContainsChrome_Linux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}

	pid, err := detectBrowserPID()
	if err != nil {
		if !strings.Contains(err.Error(), "chrome") {
			t.Errorf("Linux error should mention chrome, got: %s", err.Error())
		}
		if !strings.Contains(err.Error(), "google-chrome") {
			t.Errorf("Linux error should mention google-chrome, got: %s", err.Error())
		}
	} else if pid <= 0 {
		t.Errorf("no error but invalid PID: %d", pid)
	}
}

// ============================================
// handleOSAutomationInternal with BrowserPID=0
// ============================================

func TestOSAutomation_BrowserPID_Zero_CallsDetection(t *testing.T) {
	sec := testUploadSecurity(t)

	// Create a valid test file
	testFile := createTestFile(t, "pid-test.txt", "test content for PID detection")

	resp := handleOSAutomationInternal(OSAutomationInjectRequest{
		FilePath:   testFile,
		BrowserPID: 0,
	}, sec)

	// With BrowserPID=0, the handler should attempt PID detection instead of
	// returning the old "Missing or invalid browser_pid" error.
	if resp.Error != "" && strings.Contains(resp.Error, "Missing or invalid browser_pid") {
		t.Errorf("BrowserPID=0 should trigger auto-detection, not the old error. Got: %s", resp.Error)
	}

	// The response should either succeed (Chrome running + accessibility granted)
	// or fail with a detection/automation error (not a validation error).
	if !resp.Success && resp.Error != "" {
		// Acceptable errors: Chrome not found, accessibility denied, etc.
		// NOT acceptable: "Missing or invalid browser_pid"
		t.Logf("PID detection result (expected): %s", resp.Error)
	}
}

func TestOSAutomation_BrowserPID_Positive_SkipsDetection(t *testing.T) {
	sec := testUploadSecurity(t)
	testFile := createTestFile(t, "pid-positive.txt", "test content")

	resp := handleOSAutomationInternal(OSAutomationInjectRequest{
		FilePath:   testFile,
		BrowserPID: 99999,
	}, sec)

	// With a positive BrowserPID, detection is NOT called.
	// The request proceeds to OS automation (which will likely fail in test env,
	// but should NOT produce a "Chrome not detected" error).
	if resp.Error != "" && strings.Contains(resp.Error, "Cannot detect Chrome") {
		t.Errorf("Positive BrowserPID should skip detection. Got: %s", resp.Error)
	}
}

// ============================================
// Enhanced error messages
// ============================================

func TestOSAutomation_MacOS_ErrorIncludesTermProgram(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-only test")
	}

	// Set TERM_PROGRAM to a known value for predictable testing
	prev := os.Getenv("TERM_PROGRAM")
	os.Setenv("TERM_PROGRAM", "iTerm2")
	t.Cleanup(func() {
		if prev != "" {
			os.Setenv("TERM_PROGRAM", prev)
		} else {
			os.Unsetenv("TERM_PROGRAM")
		}
	})

	testFile := createTestFile(t, "term-test.txt", "test content")

	// Execute OS automation — it may fail (no file dialog open), which is expected.
	// We just want to verify the error message format includes TERM_PROGRAM.
	resp := executeMacOSAutomation(OSAutomationInjectRequest{
		FilePath:   testFile,
		BrowserPID: 1,
	}, time.Now())

	// If it fails (expected in test — no dialog open), check error format
	if !resp.Success && strings.Contains(resp.Error, "AppleScript failed") {
		if !strings.Contains(resp.Error, "iTerm2") {
			t.Errorf("macOS error should include TERM_PROGRAM value 'iTerm2', got: %s", resp.Error)
		}
	}
	// If it succeeds (unlikely in test), that's fine too — no assertion needed
}

func TestOSAutomation_Linux_ErrorIncludesXdotoolInstall(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}

	resp := executeLinuxAutomation(OSAutomationInjectRequest{
		FilePath:   "/tmp/test.txt",
		BrowserPID: 1,
	}, time.Now())

	if !resp.Success {
		if strings.Contains(resp.Error, "xdotool not found") {
			if !strings.Contains(resp.Error, "sudo apt install xdotool") {
				t.Errorf("xdotool missing error should include install instructions, got: %s", resp.Error)
			}
			if !strings.Contains(resp.Error, "sudo dnf install xdotool") {
				t.Errorf("xdotool missing error should include dnf install, got: %s", resp.Error)
			}
		}
	}
}
