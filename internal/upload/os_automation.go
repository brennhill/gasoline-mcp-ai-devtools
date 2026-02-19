// os_automation.go — Stage 4 OS automation: browser PID detection and platform execution.
package upload

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// HandleOSAutomation is the core logic for OS automation, testable without HTTP.
// Stage 4 requires --upload-dir.
// #lizard forgives
func HandleOSAutomation(req OSAutomationInjectRequest, sec *Security) StageResponse {
	if req.FilePath == "" {
		return StageResponse{
			Success: false,
			Stage:   4,
			Error:   "Missing required parameter: file_path",
		}
	}

	if req.BrowserPID <= 0 {
		detectedPID, err := DetectBrowserPID()
		if err != nil {
			return StageResponse{
				Success: false,
				Stage:   4,
				Error:   err.Error(),
			}
		}
		req.BrowserPID = detectedPID
	}

	// Security: full validation chain (requires upload-dir for Stage 4)
	result, err := sec.ValidateFilePath(req.FilePath, true)
	if err != nil {
		return StageResponse{
			Success: false,
			Stage:   4,
			Error:   err.Error(),
		}
	}

	// Validate path for OS automation injection safety (defense in depth)
	if err := ValidatePathForOSAutomation(result.ResolvedPath); err != nil {
		return StageResponse{
			Success: false,
			Stage:   4,
			Error:   "Invalid file path for OS automation: " + err.Error(),
		}
	}

	// Verify file exists via stat on resolved path
	if _, err := os.Stat(result.ResolvedPath); err != nil {
		if os.IsNotExist(err) {
			return StageResponse{
				Success: false,
				Stage:   4,
				Error:   "File not found: " + req.FilePath,
			}
		}
		return StageResponse{
			Success: false,
			Stage:   4,
			Error:   "Failed to access file: " + req.FilePath,
		}
	}

	// Use the resolved path for OS automation
	resolvedReq := req
	resolvedReq.FilePath = result.ResolvedPath
	return ExecuteOSAutomation(resolvedReq)
}

// DetectBrowserPID auto-detects the Chrome browser process ID using platform-specific methods.
func DetectBrowserPID() (int, error) {
	switch runtime.GOOS {
	case "darwin":
		return detectBrowserPIDDarwin()
	case "linux":
		return detectBrowserPIDLinux()
	case "windows":
		return detectBrowserPIDWindows()
	default:
		return 0, fmt.Errorf("browser PID auto-detection not supported on %s", runtime.GOOS)
	}
}

// detectBrowserPIDDarwin finds Chrome via pgrep on macOS.
func detectBrowserPIDDarwin() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "pgrep", "-x", "Google Chrome").Output()
	if err != nil {
		return 0, fmt.Errorf("Cannot detect Chrome: pgrep -x 'Google Chrome' found no process. Launch Google Chrome first")
	}
	lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
	if len(lines) == 0 || lines[0] == "" {
		return 0, fmt.Errorf("Cannot detect Chrome: pgrep -x 'Google Chrome' found no process. Launch Google Chrome first")
	}
	var pid int
	if _, err := fmt.Sscanf(lines[0], "%d", &pid); err != nil {
		return 0, fmt.Errorf("Cannot detect Chrome: pgrep -x 'Google Chrome' returned non-numeric PID %q", lines[0])
	}
	return pid, nil
}

// detectBrowserPIDLinux finds Chrome or Chromium via pgrep on Linux.
func detectBrowserPIDLinux() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, name := range []string{"chrome", "chromium", "google-chrome", "chromium-browser"} {
		out, err := exec.CommandContext(ctx, "pgrep", "-x", name).Output()
		if err != nil {
			continue
		}
		lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
		if len(lines) > 0 && lines[0] != "" {
			var pid int
			if _, err := fmt.Sscanf(lines[0], "%d", &pid); err == nil {
				return pid, nil
			}
		}
	}
	return 0, fmt.Errorf("Cannot detect Chrome: pgrep found none of 'chrome', 'chromium', 'google-chrome', 'chromium-browser'. Launch Chrome/Chromium first")
}

// detectBrowserPIDWindows finds Chrome via tasklist on Windows.
func detectBrowserPIDWindows() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tasklist", "/FI", "IMAGENAME eq chrome.exe", "/FO", "CSV", "/NH").Output()
	if err != nil {
		return 0, fmt.Errorf("Cannot detect Chrome: tasklist found no chrome.exe. Launch Google Chrome first")
	}
	lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
	if len(lines) == 0 || strings.Contains(lines[0], "No tasks") {
		return 0, fmt.Errorf("Cannot detect Chrome: tasklist found no chrome.exe. Launch Google Chrome first")
	}
	// CSV format: "chrome.exe","1234","Console","1","123,456 K"
	fields := strings.Split(lines[0], ",")
	if len(fields) < 2 {
		return 0, fmt.Errorf("Cannot detect Chrome: tasklist returned unexpected format")
	}
	pidStr := strings.Trim(fields[1], "\" ")
	var pid int
	if _, err := fmt.Sscanf(pidStr, "%d", &pid); err != nil {
		return 0, fmt.Errorf("Cannot detect Chrome: tasklist returned non-numeric PID %q", pidStr)
	}
	return pid, nil
}

// DismissFileDialog sends Escape to close a dangling native file dialog.
func DismissFileDialog() StageResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "osascript", "-e", `tell application "System Events" to key code 53`)
	case "linux":
		if _, err := exec.LookPath("xdotool"); err != nil {
			return StageResponse{Success: false, Stage: 4, Error: "xdotool not found"}
		}
		cmd = exec.CommandContext(ctx, "xdotool", "key", "Escape")
	case "windows":
		cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command",
			`Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.SendKeys]::SendWait("{ESCAPE}")`)
	default:
		return StageResponse{Success: false, Stage: 4, Error: "unsupported OS"}
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return StageResponse{Success: false, Stage: 4, Error: fmt.Sprintf("dismiss failed: %v. Output: %s", err, string(output))}
	}
	return StageResponse{Success: true, Stage: 4, Status: "file dialog dismissed"}
}

// ExecuteOSAutomation performs platform-specific OS automation.
// Caller must validate path with ValidatePathForOSAutomation before calling.
func ExecuteOSAutomation(req OSAutomationInjectRequest) StageResponse {
	start := time.Now()
	switch runtime.GOOS {
	case "darwin":
		return executeMacOSAutomation(req, start)
	case "windows":
		return executeWindowsAutomation(req, start)
	case "linux":
		return executeLinuxAutomation(req, start)
	default:
		return StageResponse{
			Success: false,
			Stage:   4,
			Error:   fmt.Sprintf("OS automation not supported on %s", runtime.GOOS),
			Suggestions: []string{
				"Use Stage 3 (form interception) instead",
				"Manually upload the file",
			},
		}
	}
}

// executeMacOSAutomation uses AppleScript to inject file path into file dialog.
func executeMacOSAutomation(req OSAutomationInjectRequest, start time.Time) StageResponse {
	safePath := SanitizeForAppleScript(req.FilePath)

	script := fmt.Sprintf(`tell application "System Events"
	delay 0.5
	keystroke "g" using {command down, shift down}
	delay 0.5
	keystroke "%s"
	delay 0.3
	key code 36
	delay 0.5
	key code 36
end tell`, safePath)

	// #nosec G204 -- script is built from sanitized file path
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "osascript", "-e", script) // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command -- input sanitized
	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := fmt.Sprintf("AppleScript failed: %v", err)
		if len(output) > 0 {
			errMsg += " Output: " + string(output)
		}
		termApp := os.Getenv("TERM_PROGRAM")
		if termApp == "" {
			termApp = "your terminal"
		}
		errMsg += fmt.Sprintf(". Fix: System Settings > Privacy & Security > Accessibility > enable %s", termApp)
		return StageResponse{
			Success:    false,
			Stage:      4,
			Error:      errMsg,
			DurationMs: time.Since(start).Milliseconds(),
			Suggestions: []string{
				fmt.Sprintf("Grant Accessibility permissions: System Settings > Privacy & Security > Accessibility > enable %s", termApp),
				"Ensure a file dialog is open in Chrome",
			},
		}
	}

	return StageResponse{
		Success:    true,
		Stage:      4,
		Status:     "OS automation: file path injected via AppleScript",
		FileName:   filepath.Base(req.FilePath),
		DurationMs: time.Since(start).Milliseconds(),
	}
}

// executeWindowsAutomation uses PowerShell with SendKeys to inject file path.
func executeWindowsAutomation(req OSAutomationInjectRequest, start time.Time) StageResponse {
	safePath := SanitizeForSendKeys(req.FilePath)
	psPath := strings.ReplaceAll(safePath, `"`, "`\"")
	script := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
Start-Sleep -Milliseconds 500
# Type the file path into the file name field
[System.Windows.Forms.SendKeys]::SendWait("%s")
Start-Sleep -Milliseconds 300
# Press Enter
[System.Windows.Forms.SendKeys]::SendWait("{ENTER}")
`, psPath)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", script) // #nosec G204 -- path sanitized
	output, err := cmd.CombinedOutput()
	if err != nil {
		return StageResponse{
			Success:    false,
			Stage:      4,
			Error:      fmt.Sprintf("PowerShell automation failed: %v. Output: %s", err, string(output)),
			DurationMs: time.Since(start).Milliseconds(),
			Suggestions: []string{
				"Ensure a file dialog is open in Chrome",
				"Run with administrator privileges if needed",
			},
		}
	}

	return StageResponse{
		Success:    true,
		Stage:      4,
		Status:     "OS automation: file path injected via PowerShell/SendKeys",
		FileName:   filepath.Base(req.FilePath),
		DurationMs: time.Since(start).Milliseconds(),
	}
}

// executeLinuxAutomation uses xdotool to inject file path into file dialog.
// #lizard forgives
func executeLinuxAutomation(req OSAutomationInjectRequest, start time.Time) StageResponse {
	if _, err := exec.LookPath("xdotool"); err != nil {
		return StageResponse{
			Success: false,
			Stage:   4,
			Error:   "xdotool not found. Install: sudo apt install xdotool (Debian/Ubuntu) or sudo dnf install xdotool (Fedora).",
			Suggestions: []string{
				"Install xdotool: sudo apt install xdotool (Debian/Ubuntu)",
				"Install xdotool: sudo dnf install xdotool (Fedora)",
				"Use Stage 3 (form interception) instead",
			},
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	commands := []struct {
		name string
		args []string
	}{
		{"xdotool", []string{"search", "--name", "Open", "windowactivate"}},
		{"xdotool", []string{"key", "ctrl+l"}},
		{"xdotool", []string{"type", "--clearmodifiers", "--", req.FilePath}},
		{"xdotool", []string{"key", "Return"}},
	}

	for _, c := range commands {
		cmd := exec.CommandContext(ctx, c.name, c.args...) // #nosec G204 -- xdotool path from LookPath
		if output, err := cmd.CombinedOutput(); err != nil {
			return StageResponse{
				Success:    false,
				Stage:      4,
				Error:      fmt.Sprintf("xdotool command failed: %v. Output: %s", err, string(output)),
				DurationMs: time.Since(start).Milliseconds(),
				Suggestions: []string{
					"Ensure a file dialog is open",
					"Check that X11/Wayland session is active",
				},
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	return StageResponse{
		Success:    true,
		Stage:      4,
		Status:     "OS automation: file path injected via xdotool",
		FileName:   filepath.Base(req.FilePath),
		DurationMs: time.Since(start).Milliseconds(),
	}
}
