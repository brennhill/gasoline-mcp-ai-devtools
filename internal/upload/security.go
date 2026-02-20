// security.go — Folder-scoped permissions and sensitive path denylist for file upload.
// Implements the validation chain: Clean → IsAbs → EvalSymlinks → denylist → upload-dir check.
package upload

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

// Security holds the validated upload security configuration.
// Created at startup, immutable after initialization.
type Security struct {
	// uploadDir is the resolved absolute path to the allowed upload directory.
	// Empty string means no upload-dir was specified (Stage 1 still works with denylist).
	uploadDir string

	// userDenyPatterns are additional patterns from --upload-deny-pattern flags.
	userDenyPatterns []string
}

// NewSecurity creates a Security with the given upload directory and deny patterns.
// For production use, prefer ValidateUploadDir which also validates the directory.
// This constructor is useful for tests that need direct control.
func NewSecurity(uploadDir string, userDenyPatterns []string) *Security {
	return &Security{uploadDir: uploadDir, userDenyPatterns: userDenyPatterns}
}

// ============================================
// Startup Validation
// ============================================

// ValidateUploadDir validates the --upload-dir flag at startup.
// Returns a configured Security or an error that should halt startup.
func ValidateUploadDir(rawDir string, userDenyPatterns []string) (*Security, error) {
	sec := &Security{userDenyPatterns: userDenyPatterns}

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
		return "", fmt.Errorf("--upload-dir does not exist: %s: %w", rawDir, err)
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

	if pattern, matched := MatchesDenylist(resolved); matched {
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
func (s *Security) ValidateFilePath(rawPath string, requireUploadDir bool) (*PathValidationResult, error) {
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
			return "", fmt.Errorf("file not found: %s: %w", rawPath, err)
		}
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	return resolved, nil
}

// checkDenylists checks the resolved path against built-in and user deny patterns.
func (s *Security) checkDenylists(rawPath, resolved string) error {
	if pattern, matched := MatchesDenylist(resolved); matched {
		return &PathDeniedError{Path: rawPath, Pattern: pattern, UploadDir: s.uploadDir}
	}
	if pattern, matched := MatchesUserDenylist(resolved, s.userDenyPatterns); matched {
		return &PathDeniedError{Path: rawPath, Pattern: pattern, UploadDir: s.uploadDir}
	}
	return nil
}

// checkUploadDirConstraint verifies the file is within --upload-dir when required.
func (s *Security) checkUploadDirConstraint(rawPath, resolved string, required bool) error {
	if !required {
		return nil
	}
	if s.uploadDir == "" {
		return &UploadDirRequiredError{}
	}
	if !IsWithinDir(resolved, s.uploadDir) {
		return &PathDeniedError{Path: rawPath, Pattern: "outside --upload-dir", UploadDir: s.uploadDir}
	}
	return nil
}

// IsWithinDir checks if filePath is within or equal to dirPath.
// Uses case-insensitive comparison on macOS/Windows.
func IsWithinDir(filePath, dirPath string) bool {
	// Ensure dirPath ends with separator for prefix matching
	dirWithSep := dirPath
	if !strings.HasSuffix(dirWithSep, string(filepath.Separator)) {
		dirWithSep += string(filepath.Separator)
	}
	return PathHasPrefixFold(filePath, dirWithSep) || PathsEqualFold(filePath, dirPath)
}

// checkHardlink is defined in security_unix.go and security_windows.go

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

// BuiltinDenyPatterns are hardcoded sensitive path patterns that cannot be removed.
// Patterns use filepath.Match syntax. Prefix ~ is expanded to the user's home directory.
var BuiltinDenyPatterns []DenyPattern

// DenyPattern represents a sensitive path pattern.
type DenyPattern struct {
	Prefix  string // resolved absolute path prefix to match
	Display string // display pattern (with ~ for error messages)
	Desc    string // human-readable description
}

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
			BuiltinDenyPatterns = append(BuiltinDenyPatterns, DenyPattern{
				Prefix:  expanded,
				Display: r.pattern,
				Desc:    r.desc,
			})
		}
	}

	// Absolute system paths (always active, no $HOME dependency).
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
		BuiltinDenyPatterns = append(BuiltinDenyPatterns, DenyPattern{
			Prefix:  filepath.Clean(sp.path),
			Display: sp.path,
			Desc:    sp.desc,
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
			BuiltinDenyPatterns = append(BuiltinDenyPatterns, DenyPattern{
				Prefix:  filepath.Clean(wp.path),
				Display: wp.path,
				Desc:    wp.desc,
			})
		}
	}
}

// PathsEqualFold returns true if a == b, using case-insensitive comparison
// on macOS and Windows (which have case-insensitive filesystems by default).
func PathsEqualFold(a, b string) bool {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// PathHasPrefixFold returns true if s starts with prefix, using case-insensitive
// comparison on macOS and Windows.
func PathHasPrefixFold(s, prefix string) bool {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
	}
	return strings.HasPrefix(s, prefix)
}

// SensitiveExtensions maps file extensions to their denylist pattern display string.
var SensitiveExtensions = map[string]string{
	".pem": "**/*.pem", ".key": "**/*.key",
	".p12": "**/*.p12", ".pfx": "**/*.pfx",
	".keystore": "**/*.keystore",
}

// MatchesDenylist checks if a resolved path matches any built-in deny pattern.
// Returns the matching display pattern and true if denied.
func MatchesDenylist(resolvedPath string) (string, bool) {
	if pattern, matched := matchesPrefixDenylist(resolvedPath); matched {
		return pattern, true
	}
	return matchesBasenameDenylist(resolvedPath)
}

// matchesPrefixDenylist checks against directory-prefix-based deny patterns.
func matchesPrefixDenylist(resolvedPath string) (string, bool) {
	for _, dp := range BuiltinDenyPatterns {
		if PathsEqualFold(resolvedPath, dp.Prefix) || PathHasPrefixFold(resolvedPath, dp.Prefix+string(filepath.Separator)) {
			return dp.Display, true
		}
	}
	return "", false
}

// matchesBasenameDenylist checks against filename-based deny patterns.
func matchesBasenameDenylist(resolvedPath string) (string, bool) {
	baseLower := strings.ToLower(filepath.Base(resolvedPath))

	if baseLower == ".env" || strings.HasPrefix(baseLower, ".env.") {
		return "**/.env*", true
	}

	ext := strings.ToLower(filepath.Ext(resolvedPath))
	if pattern, ok := SensitiveExtensions[ext]; ok {
		return pattern, true
	}

	dir := filepath.Dir(resolvedPath)
	if strings.EqualFold(filepath.Base(dir), ".git") && baseLower == "config" {
		return "**/.git/config", true
	}

	return "", false
}

// MatchesUserDenylist checks if a resolved path matches any user-defined deny pattern.
func MatchesUserDenylist(resolvedPath string, patterns []string) (string, bool) {
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
