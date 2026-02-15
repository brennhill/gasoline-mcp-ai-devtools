// upload_security.go — Folder-scoped permissions and sensitive path denylist for file upload.
// Implements the validation chain: Clean → IsAbs → EvalSymlinks → denylist → upload-dir check.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ============================================
// Upload Security Configuration
// ============================================

// UploadSecurity holds the validated upload security configuration.
// Created at startup, immutable after initialization.
type UploadSecurity struct {
	// uploadDir is the resolved absolute path to the allowed upload directory.
	// Empty string means no upload-dir was specified (Stage 1 still works with denylist).
	uploadDir string

	// userDenyPatterns are additional patterns from --upload-deny-pattern flags.
	userDenyPatterns []string
}

// ============================================
// Startup Validation
// ============================================

// ValidateUploadDir validates the --upload-dir flag at startup.
// Returns a configured UploadSecurity or an error that should halt startup.
func ValidateUploadDir(rawDir string, userDenyPatterns []string) (*UploadSecurity, error) {
	sec := &UploadSecurity{userDenyPatterns: userDenyPatterns}

	if rawDir == "" {
		return sec, nil
	}

	resolved, err := resolveAndValidateDir(rawDir)
	if err != nil {
		return nil, err
	}

	sec.uploadDir = resolved
	return sec, nil
}

// resolveAndValidateDir performs all validation steps on the upload directory path.
func resolveAndValidateDir(rawDir string) (string, error) {
	if !filepath.IsAbs(rawDir) {
		return "", fmt.Errorf("--upload-dir must be an absolute path, got: %s", rawDir)
	}

	info, err := os.Stat(rawDir)
	if err != nil {
		return "", fmt.Errorf("--upload-dir does not exist: %s", rawDir)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("--upload-dir is not a directory: %s", rawDir)
	}

	resolved, err := filepath.EvalSymlinks(rawDir)
	if err != nil {
		return "", fmt.Errorf("--upload-dir: failed to resolve symlinks: %w", err)
	}
	if resolved != filepath.Clean(rawDir) {
		return "", fmt.Errorf("--upload-dir must not be a symlink: %s resolves to %s", rawDir, resolved)
	}

	if err := rejectRootOrHome(resolved); err != nil {
		return "", fmt.Errorf("--upload-dir %s: %w", rawDir, err)
	}

	if pattern, matched := matchesDenylist(resolved); matched {
		return "", fmt.Errorf("--upload-dir matches sensitive path pattern %q: %s", pattern, rawDir)
	}

	return resolved, nil
}

// rejectRootOrHome rejects paths that are filesystem roots or user home directories.
func rejectRootOrHome(resolved string) error {
	// Reject filesystem roots
	roots := []string{"/", "/home", "/Users", "/root"}
	if runtime.GOOS == "windows" {
		// Add common Windows roots
		for _, d := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
			roots = append(roots, fmt.Sprintf("%c:\\", d))
			roots = append(roots, fmt.Sprintf("%c:\\Users", d))
		}
	}
	for _, root := range roots {
		if strings.EqualFold(resolved, filepath.Clean(root)) {
			return fmt.Errorf("must be a subdirectory, not a root or system directory")
		}
	}

	// Reject user home directory itself
	home, err := os.UserHomeDir()
	if err == nil {
		homeResolved, err := filepath.EvalSymlinks(home)
		if err == nil && resolved == homeResolved {
			return fmt.Errorf("must be a subdirectory of your home directory, not the home directory itself")
		}
	}

	return nil
}

// ============================================
// Per-Request File Path Validation
// ============================================

// PathValidationResult contains the validated, resolved file path.
type PathValidationResult struct {
	// ResolvedPath is the cleaned, symlink-resolved absolute path safe to open.
	ResolvedPath string
}

// ValidateFilePath runs the full validation chain on a file path.
// requireUploadDir controls whether the path must be within --upload-dir.
// Stage 1 passes requireUploadDir=false; Stages 2-4 pass requireUploadDir=true.
func (s *UploadSecurity) ValidateFilePath(rawPath string, requireUploadDir bool) (*PathValidationResult, error) {
	resolved, err := resolveCleanPath(rawPath)
	if err != nil {
		return nil, err
	}

	if err := s.checkDenylists(rawPath, resolved); err != nil {
		return nil, err
	}

	if err := s.checkUploadDirConstraint(rawPath, resolved, requireUploadDir); err != nil {
		return nil, err
	}

	return &PathValidationResult{ResolvedPath: resolved}, nil
}

// resolveCleanPath cleans the path, verifies it is absolute, and resolves symlinks.
func resolveCleanPath(rawPath string) (string, error) {
	cleaned := filepath.Clean(rawPath)
	if !filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("file_path must be an absolute path. Relative paths are not allowed for security")
	}
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", rawPath)
		}
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	return resolved, nil
}

// checkDenylists checks the resolved path against built-in and user deny patterns.
func (s *UploadSecurity) checkDenylists(rawPath, resolved string) error {
	if pattern, matched := matchesDenylist(resolved); matched {
		return &PathDeniedError{Path: rawPath, Pattern: pattern, UploadDir: s.uploadDir}
	}
	if pattern, matched := matchesUserDenylist(resolved, s.userDenyPatterns); matched {
		return &PathDeniedError{Path: rawPath, Pattern: pattern, UploadDir: s.uploadDir}
	}
	return nil
}

// checkUploadDirConstraint verifies the file is within --upload-dir when required.
func (s *UploadSecurity) checkUploadDirConstraint(rawPath, resolved string, required bool) error {
	if !required {
		return nil
	}
	if s.uploadDir == "" {
		return &UploadDirRequiredError{}
	}
	if !isWithinDir(resolved, s.uploadDir) {
		return &PathDeniedError{Path: rawPath, Pattern: "outside --upload-dir", UploadDir: s.uploadDir}
	}
	return nil
}

// isWithinDir checks if filePath is within or equal to dirPath.
// Uses case-insensitive comparison on macOS/Windows.
func isWithinDir(filePath, dirPath string) bool {
	// Ensure dirPath ends with separator for prefix matching
	dirWithSep := dirPath
	if !strings.HasSuffix(dirWithSep, string(filepath.Separator)) {
		dirWithSep += string(filepath.Separator)
	}
	return pathHasPrefixFold(filePath, dirWithSep) || pathsEqualFold(filePath, dirPath)
}

// checkHardlink is defined in upload_security_unix.go and upload_security_windows.go

// ============================================
// Error Types
// ============================================

// PathDeniedError is returned when a file path is blocked by the denylist or upload-dir constraint.
type PathDeniedError struct {
	Path      string
	Pattern   string
	UploadDir string
}

func (e *PathDeniedError) Error() string {
	if e.Pattern == "outside --upload-dir" {
		return fmt.Sprintf("File path %q is outside the allowed upload directory (%s). Move the file there and retry.", e.Path, e.UploadDir)
	}
	msg := fmt.Sprintf("File path %q is not allowed: matches sensitive path pattern %q.", e.Path, e.Pattern)
	if e.UploadDir != "" {
		msg += fmt.Sprintf(" Move the file to your upload directory (%s) and retry.", e.UploadDir)
	}
	return msg
}

// UploadDirRequiredError is returned when Stages 2-4 are attempted without --upload-dir.
type UploadDirRequiredError struct{}

func (e *UploadDirRequiredError) Error() string {
	return "This upload stage requires --upload-dir to be set. Restart the server with --upload-dir=/path/to/folder and move your files there."
}

// ============================================
// Sensitive Path Denylist
// ============================================

// builtinDenyPatterns are hardcoded sensitive path patterns that cannot be removed.
// Patterns use filepath.Match syntax. Prefix ~ is expanded to the user's home directory.
var builtinDenyPatterns []denyPattern

// #lizard forgives
func init() {
	home, _ := os.UserHomeDir()

	// Home-relative patterns (only added if $HOME is available)
	if home != "" {
		homePatterns := []struct {
			pattern string
			desc    string
		}{
			// SSH & GPG keys
			{"~/.ssh", "SSH directory"},
			{"~/.gnupg", "GPG directory"},

			// Cloud credentials
			{"~/.aws", "AWS credentials"},
			{"~/.config/gcloud", "GCP credentials"},
			{"~/.azure", "Azure credentials"},
			{"~/.config/doctl", "DigitalOcean credentials"},
			{"~/.kube", "Kubernetes config"},

			// Shell history
			{"~/.bash_history", "shell history"},
			{"~/.zsh_history", "shell history"},
			{"~/.node_repl_history", "shell history"},
			{"~/.python_history", "shell history"},

			// Browser data
			{"~/Library/Application Support/Google/Chrome", "browser data"},
			{"~/.config/google-chrome", "browser data"},
			{"~/.config/chromium", "browser data"},
			{"~/Library/Application Support/Firefox", "browser data"},
			{"~/.mozilla/firefox", "browser data"},

			// Package manager auth
			{"~/.npmrc", "npm credentials"},
			{"~/.pypirc", "PyPI credentials"},
			{"~/.docker/config.json", "Docker credentials"},
			{"~/.config/gh/hosts.yml", "GitHub CLI credentials"},
		}
		for _, r := range homePatterns {
			expanded := strings.Replace(r.pattern, "~", home, 1)
			expanded = filepath.Clean(expanded)
			builtinDenyPatterns = append(builtinDenyPatterns, denyPattern{
				prefix:  expanded,
				display: r.pattern,
				desc:    r.desc,
			})
		}
	}

	// Absolute system paths (always active, no $HOME dependency).
	// These cover common sensitive locations on all Unix-like systems,
	// including containers and CI where $HOME may be unset.
	systemPaths := []struct {
		path string
		desc string
	}{
		// System files
		{"/etc/shadow", "system password file"},
		{"/etc/passwd", "system user file"},
		{"/etc/sudoers", "sudoers file"},
		{"/proc", "proc filesystem"},
		{"/sys", "sys filesystem"},

		// Root user credentials (covers containers running as root)
		{"/root/.ssh", "root SSH directory"},
		{"/root/.gnupg", "root GPG directory"},
		{"/root/.aws", "root AWS credentials"},
		{"/root/.config/gcloud", "root GCP credentials"},
		{"/root/.azure", "root Azure credentials"},
		{"/root/.kube", "root Kubernetes config"},
		{"/root/.npmrc", "root npm credentials"},
		{"/root/.docker", "root Docker config"},
		{"/root/.bash_history", "root shell history"},

		// macOS system keychain
		{"/Library/Keychains", "system keychain"},
	}
	for _, sp := range systemPaths {
		builtinDenyPatterns = append(builtinDenyPatterns, denyPattern{
			prefix:  filepath.Clean(sp.path),
			display: sp.path,
			desc:    sp.desc,
		})
	}

	// Windows system paths
	if runtime.GOOS == "windows" {
		winPaths := []struct {
			path string
			desc string
		}{
			{`C:\Windows\System32\config`, "Windows registry hives"},
			{`C:\Windows\System32\drivers\etc`, "Windows hosts file"},
		}
		for _, wp := range winPaths {
			builtinDenyPatterns = append(builtinDenyPatterns, denyPattern{
				prefix:  filepath.Clean(wp.path),
				display: wp.path,
				desc:    wp.desc,
			})
		}
	}
}

type denyPattern struct {
	prefix  string // resolved absolute path prefix to match
	display string // display pattern (with ~ for error messages)
	desc    string // human-readable description
}

// pathsEqualFold returns true if a == b, using case-insensitive comparison
// on macOS and Windows (which have case-insensitive filesystems by default).
func pathsEqualFold(a, b string) bool {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// pathHasPrefixFold returns true if s starts with prefix, using case-insensitive
// comparison on macOS and Windows.
func pathHasPrefixFold(s, prefix string) bool {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
	}
	return strings.HasPrefix(s, prefix)
}

// sensitiveExtensions maps file extensions to their denylist pattern display string.
var sensitiveExtensions = map[string]string{
	".pem": "**/*.pem", ".key": "**/*.key",
	".p12": "**/*.p12", ".pfx": "**/*.pfx",
	".keystore": "**/*.keystore",
}

// matchesDenylist checks if a resolved path matches any built-in deny pattern.
// Returns the matching display pattern and true if denied.
// Uses case-insensitive matching on macOS/Windows (case-insensitive filesystems).
func matchesDenylist(resolvedPath string) (string, bool) {
	if pattern, matched := matchesPrefixDenylist(resolvedPath); matched {
		return pattern, true
	}
	return matchesBasenameDenylist(resolvedPath)
}

// matchesPrefixDenylist checks against directory-prefix-based deny patterns.
func matchesPrefixDenylist(resolvedPath string) (string, bool) {
	for _, dp := range builtinDenyPatterns {
		if pathsEqualFold(resolvedPath, dp.prefix) || pathHasPrefixFold(resolvedPath, dp.prefix+string(filepath.Separator)) {
			return dp.display, true
		}
	}
	return "", false
}

// matchesBasenameDenylist checks against filename-based deny patterns (.env, private keys, .git/config).
func matchesBasenameDenylist(resolvedPath string) (string, bool) {
	baseLower := strings.ToLower(filepath.Base(resolvedPath))

	if baseLower == ".env" || strings.HasPrefix(baseLower, ".env.") {
		return "**/.env*", true
	}

	ext := strings.ToLower(filepath.Ext(resolvedPath))
	if pattern, ok := sensitiveExtensions[ext]; ok {
		return pattern, true
	}

	dir := filepath.Dir(resolvedPath)
	if strings.EqualFold(filepath.Base(dir), ".git") && baseLower == "config" {
		return "**/.git/config", true
	}

	return "", false
}

// matchesUserDenylist checks if a resolved path matches any user-defined deny pattern.
func matchesUserDenylist(resolvedPath string, patterns []string) (string, bool) {
	for _, pattern := range patterns {
		// Try filepath.Match on the full path
		if matched, _ := filepath.Match(pattern, resolvedPath); matched {
			return pattern, true
		}
		// Try matching just the basename (for patterns like "*.sqlite")
		if matched, _ := filepath.Match(pattern, filepath.Base(resolvedPath)); matched {
			return pattern, true
		}
	}
	return "", false
}
