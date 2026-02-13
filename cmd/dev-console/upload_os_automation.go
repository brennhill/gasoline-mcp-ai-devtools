// upload_os_automation.go — Stage 4 OS automation, security validators, and input sanitizers.
package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// handleOSAutomationInternal is the core logic for OS automation, testable without HTTP.
// Stage 4 requires --upload-dir.
// #lizard forgives
func handleOSAutomationInternal(req OSAutomationInjectRequest, sec *UploadSecurity) UploadStageResponse {
	if req.FilePath == "" {
		return UploadStageResponse{
			Success: false,
			Stage:   4,
			Error:   "Missing required parameter: file_path",
		}
	}

	if req.BrowserPID <= 0 {
		return UploadStageResponse{
			Success: false,
			Stage:   4,
			Error:   "Missing or invalid browser_pid. Provide the Chrome browser process ID.",
		}
	}

	// Security: full validation chain (requires upload-dir for Stage 4)
	result, err := sec.ValidateFilePath(req.FilePath, true)
	if err != nil {
		return UploadStageResponse{
			Success: false,
			Stage:   4,
			Error:   err.Error(),
		}
	}

	// Validate path for OS automation injection safety (defense in depth)
	if err := validatePathForOSAutomation(result.ResolvedPath); err != nil {
		return UploadStageResponse{
			Success: false,
			Stage:   4,
			Error:   "Invalid file path for OS automation: " + err.Error(),
		}
	}

	// Verify file exists via stat on resolved path
	if _, err := os.Stat(result.ResolvedPath); err != nil {
		if os.IsNotExist(err) {
			return UploadStageResponse{
				Success: false,
				Stage:   4,
				Error:   "File not found: " + req.FilePath,
			}
		}
		return UploadStageResponse{
			Success: false,
			Stage:   4,
			Error:   "Failed to access file: " + req.FilePath,
		}
	}

	// Use the resolved path for OS automation
	resolvedReq := req
	resolvedReq.FilePath = result.ResolvedPath
	return executeOSAutomation(resolvedReq)
}

// handleOSAutomationInternalMethod is the ToolHandler method wrapper for testing
func (h *ToolHandler) handleOSAutomationInternal(req OSAutomationInjectRequest) UploadStageResponse {
	return handleOSAutomationInternal(req, h.uploadSecurity)
}

// validatePathForOSAutomation rejects file paths containing shell metacharacters
// that could be used for command injection in OS automation scripts.
func validatePathForOSAutomation(filePath string) error {
	// Reject null bytes (path traversal via null byte injection)
	if strings.ContainsRune(filePath, 0) {
		return fmt.Errorf("file path contains null byte")
	}
	// Reject newlines (can break AppleScript/PowerShell script structure)
	if strings.ContainsAny(filePath, "\n\r") {
		return fmt.Errorf("file path contains newline characters")
	}
	// Reject backticks (shell command substitution in PowerShell)
	if strings.Contains(filePath, "`") {
		return fmt.Errorf("file path contains backtick characters")
	}
	return nil
}

// executeOSAutomation performs platform-specific OS automation.
// Caller must validate path with validatePathForOSAutomation before calling.
func executeOSAutomation(req OSAutomationInjectRequest) UploadStageResponse {
	start := time.Now()
	switch runtime.GOOS {
	case "darwin":
		return executeMacOSAutomation(req, start)
	case "windows":
		return executeWindowsAutomation(req, start)
	case "linux":
		return executeLinuxAutomation(req, start)
	default:
		return UploadStageResponse{
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

// ============================================
// Security Validators
// ============================================

// ssrfAllowedHostsList holds host or host:port values that bypass SSRF checks.
// Set via --ssrf-allow-host flag (repeatable). Intended for test use only.
var ssrfAllowedHostsList []string

// isSSRFAllowedHost returns true if hostOrAddr matches an --ssrf-allow-host entry.
func isSSRFAllowedHost(hostOrAddr string) bool {
	for _, allowed := range ssrfAllowedHostsList {
		if allowed == hostOrAddr {
			return true
		}
	}
	return false
}

// allowedHTTPMethods is the set of HTTP methods permitted for form submission.
var allowedHTTPMethods = map[string]bool{
	"POST":  true,
	"PUT":   true,
	"PATCH": true,
}

// validateHTTPMethod checks that the method is in the allowlist.
func validateHTTPMethod(method string) error {
	upper := strings.ToUpper(method)
	if !allowedHTTPMethods[upper] {
		return fmt.Errorf("HTTP method %q is not allowed. Use POST, PUT, or PATCH", method)
	}
	return nil
}

// skipSSRFCheck disables private IP blocking in tests where httptest.NewServer
// uses 127.0.0.1. Must only be set from test code.
var skipSSRFCheck bool

// validateFormActionURL validates the form submission target URL to prevent SSRF.
// Only allows http/https schemes and blocks requests to private/reserved IP ranges.
func validateFormActionURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("malformed URL: %w", err)
	}

	// Only allow http and https schemes
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("scheme %q not allowed. Only http and https are permitted", u.Scheme)
	}

	// Resolve the hostname to check for private IPs
	hostname := u.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL has no hostname")
	}

	// Check --ssrf-allow-host flag (test use: allows localhost test servers)
	hostPort := hostname
	if u.Port() != "" {
		hostPort = hostname + ":" + u.Port()
	}
	if isSSRFAllowedHost(hostPort) || isSSRFAllowedHost(hostname) {
		return nil
	}

	// Block well-known loopback/metadata hostnames
	lowerHost := strings.ToLower(hostname)
	if lowerHost == "localhost" || lowerHost == "metadata.google.internal" {
		return fmt.Errorf("hostname %q is not allowed", hostname)
	}

	// In test mode, allow private IPs (httptest.NewServer uses 127.0.0.1)
	if skipSSRFCheck {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), ssrfLookupTimeout)
	defer cancel()

	if _, err := resolvePublicIP(ctx, hostname); err != nil {
		return err
	}

	return nil
}

// privateRanges is parsed once at init for efficient SSRF checks.
var privateRanges []*net.IPNet

func init() {
	for _, cidr := range []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC 1918
		"172.16.0.0/12",  // RFC 1918
		"192.168.0.0/16", // RFC 1918
		"169.254.0.0/16", // link-local / cloud metadata
		"0.0.0.0/8",      // unspecified (routes to localhost)
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local
		"fe80::/10",      // IPv6 link-local
	} {
		_, ipNet, _ := net.ParseCIDR(cidr)
		privateRanges = append(privateRanges, ipNet)
	}
}

// isPrivateIP returns true if the IP is in a private, loopback, link-local, or unspecified range.
func isPrivateIP(ip net.IP) bool {
	if ip.IsUnspecified() || ip.IsLoopback() {
		return true
	}
	for _, cidr := range privateRanges {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// validateCookieHeader rejects cookie values containing header injection characters.
func validateCookieHeader(cookies string) error {
	if cookies == "" {
		return nil
	}
	if strings.ContainsAny(cookies, "\r\n\x00") {
		return fmt.Errorf("cookies contain invalid characters (newline or null byte)")
	}
	return nil
}

// ============================================
// Input Sanitizers
// ============================================

// sanitizeForContentDisposition removes characters that could break Content-Disposition
// header framing (quotes, newlines, null bytes). Prevents multipart header injection.
func sanitizeForContentDisposition(s string) string {
	s = strings.ReplaceAll(s, `"`, "_")
	s = strings.ReplaceAll(s, "\n", "_")
	s = strings.ReplaceAll(s, "\r", "_")
	s = strings.ReplaceAll(s, "\x00", "_")
	return s
}

// sanitizeForAppleScript escapes a string for safe embedding in AppleScript.
// Replaces backslashes and double quotes to prevent command injection.
func sanitizeForAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// executeMacOSAutomation uses AppleScript to inject file path into file dialog
func executeMacOSAutomation(req OSAutomationInjectRequest, start time.Time) UploadStageResponse {
	// Sanitize file path to prevent AppleScript injection
	safePath := sanitizeForAppleScript(req.FilePath)

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

	// #nosec G204 -- script is built from sanitized file path (absolute path check + escaping above)
	cmd := exec.Command("osascript", "-e", script) // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command -- input sanitized by sanitizeForAppleScript
	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := fmt.Sprintf("AppleScript failed: %v", err)
		if len(output) > 0 {
			errMsg += " Output: " + string(output)
		}
		return UploadStageResponse{
			Success:    false,
			Stage:      4,
			Error:      errMsg,
			DurationMs: time.Since(start).Milliseconds(),
			Suggestions: []string{
				"Grant Accessibility permissions: System Settings > Privacy & Security > Accessibility",
				"Ensure a file dialog is open in Chrome",
			},
		}
	}

	return UploadStageResponse{
		Success:    true,
		Stage:      4,
		Status:     "OS automation: file path injected via AppleScript",
		FileName:   filepath.Base(req.FilePath),
		DurationMs: time.Since(start).Milliseconds(),
	}
}

// sanitizeForSendKeys escapes a string for safe use with SendKeys.
// SendKeys treats +, ^, %, ~, (, ), {, } as special characters.
func sanitizeForSendKeys(s string) string {
	replacer := strings.NewReplacer(
		"+", "{+}",
		"^", "{^}",
		"%", "{%}",
		"~", "{~}",
		"(", "{(}",
		")", "{)}",
		"{", "{{}",
		"}", "{}}",
	)
	return replacer.Replace(s)
}

// executeWindowsAutomation uses PowerShell with SendKeys to inject file path
func executeWindowsAutomation(req OSAutomationInjectRequest, start time.Time) UploadStageResponse {
	// Sanitize file path for SendKeys (escape special characters)
	safePath := sanitizeForSendKeys(req.FilePath)

	// Two layers of escaping:
	// 1. sanitizeForSendKeys: escapes SendKeys special chars (+^%~(){})
	// 2. Quote escape: backtick-escapes " for PowerShell string literal
	// validatePathForOSAutomation (called before executeOSAutomation) already
	// rejects null bytes, newlines, and backticks — so no PS code injection.
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

	cmd := exec.Command("powershell", "-NoProfile", "-Command", script) // #nosec G204 -- path sanitized
	output, err := cmd.CombinedOutput()
	if err != nil {
		return UploadStageResponse{
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

	return UploadStageResponse{
		Success:    true,
		Stage:      4,
		Status:     "OS automation: file path injected via PowerShell/SendKeys",
		FileName:   filepath.Base(req.FilePath),
		DurationMs: time.Since(start).Milliseconds(),
	}
}

// executeLinuxAutomation uses xdotool to inject file path into file dialog
// #lizard forgives
func executeLinuxAutomation(req OSAutomationInjectRequest, start time.Time) UploadStageResponse {
	// Check if xdotool is available
	if _, err := exec.LookPath("xdotool"); err != nil {
		return UploadStageResponse{
			Success: false,
			Stage:   4,
			Error:   "xdotool not found. Install with: sudo apt install xdotool",
			Suggestions: []string{
				"Install xdotool: sudo apt install xdotool (Debian/Ubuntu)",
				"Install xdotool: sudo dnf install xdotool (Fedora)",
				"Use Stage 3 (form interception) instead",
			},
		}
	}

	// xdotool 'type' sends raw X11 key events — no shell interpretation.
	// validatePathForOSAutomation (called before executeOSAutomation) already
	// rejects null bytes, newlines, and backticks. Use '--' argument terminator
	// to prevent paths from being misinterpreted as xdotool flags.
	commands := []struct {
		name string
		args []string
	}{
		{"xdotool", []string{"search", "--name", "Open", "windowactivate"}},
		{"xdotool", []string{"key", "ctrl+l"}},                                // Focus location bar
		{"xdotool", []string{"type", "--clearmodifiers", "--", req.FilePath}}, // Type path (-- prevents flag injection)
		{"xdotool", []string{"key", "Return"}},                                // Confirm
	}

	for _, c := range commands {
		cmd := exec.Command(c.name, c.args...) // #nosec G204 -- xdotool path from LookPath // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command, go_subproc_rule-subproc -- OS automation executes user-requested xdotool command
		if output, err := cmd.CombinedOutput(); err != nil {
			return UploadStageResponse{
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
		time.Sleep(200 * time.Millisecond) // Brief pause between commands
	}

	return UploadStageResponse{
		Success:    true,
		Stage:      4,
		Status:     "OS automation: file path injected via xdotool",
		FileName:   filepath.Base(req.FilePath),
		DurationMs: time.Since(start).Milliseconds(),
	}
}
