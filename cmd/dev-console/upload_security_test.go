// upload_security_test.go — Tests for folder-scoped permissions and sensitive path denylist.
// Covers: denylist matching, symlink resolution, path traversal, upload-dir enforcement,
// startup validation, denied path error messages, Stage 1 vs Stage 2-4 behavior.
//
// Run: go test ./cmd/dev-console -run "TestUploadSec" -v
package main

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ============================================
// 1. Denylist Matching
// ============================================

func TestUploadSec_Denylist_SSHKeys(t *testing.T) {
	home, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "config"),
	}
	for _, p := range paths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			pattern, matched := matchesDenylist(p)
			if !matched {
				t.Errorf("matchesDenylist(%q) should match SSH directory", p)
			}
			if !strings.Contains(pattern, ".ssh") {
				t.Errorf("pattern should mention .ssh, got %q", pattern)
			}
		})
	}
}

func TestUploadSec_Denylist_AWSCredentials(t *testing.T) {
	home, _ := os.UserHomeDir()
	p := filepath.Join(home, ".aws", "credentials")
	pattern, matched := matchesDenylist(p)
	if !matched {
		t.Errorf("matchesDenylist(%q) should match AWS credentials", p)
	}
	if !strings.Contains(pattern, ".aws") {
		t.Errorf("pattern should mention .aws, got %q", pattern)
	}
}

func TestUploadSec_Denylist_EnvFiles(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"dotenv", "/app/project/.env"},
		{"dotenv_local", "/app/project/.env.local"},
		{"dotenv_production", "/app/project/.env.production"},
		{"nested", "/deep/nested/dir/.env"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, matched := matchesDenylist(tc.path)
			if !matched {
				t.Errorf("matchesDenylist(%q) should match .env pattern", tc.path)
			}
		})
	}
}

func TestUploadSec_Denylist_KeyFiles(t *testing.T) {
	exts := []string{".pem", ".key", ".p12", ".pfx", ".keystore"}
	for _, ext := range exts {
		t.Run(ext, func(t *testing.T) {
			p := "/some/path/server" + ext
			_, matched := matchesDenylist(p)
			if !matched {
				t.Errorf("matchesDenylist(%q) should match key extension", p)
			}
		})
	}
}

func TestUploadSec_Denylist_GitConfig(t *testing.T) {
	p := "/app/repo/.git/config"
	_, matched := matchesDenylist(p)
	if !matched {
		t.Errorf("matchesDenylist(%q) should match .git/config", p)
	}
}

func TestUploadSec_Denylist_SystemFiles(t *testing.T) {
	paths := []string{"/etc/shadow", "/etc/passwd", "/etc/sudoers"}
	for _, p := range paths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			_, matched := matchesDenylist(p)
			if !matched {
				t.Errorf("matchesDenylist(%q) should match system file", p)
			}
		})
	}
}

func TestUploadSec_Denylist_SafePaths(t *testing.T) {
	safe := []string{
		"/Users/brenn/Videos/video.mp4",
		"/tmp/upload/report.pdf",
		"/home/user/documents/photo.jpg",
		"/Users/brenn/Uploads/data.csv",
	}
	for _, p := range safe {
		t.Run(filepath.Base(p), func(t *testing.T) {
			_, matched := matchesDenylist(p)
			if matched {
				t.Errorf("matchesDenylist(%q) should NOT match (safe path)", p)
			}
		})
	}
}

// ============================================
// 2. User Deny Patterns
// ============================================

func TestUploadSec_UserDenyPatterns(t *testing.T) {
	patterns := []string{"*.sqlite", "company-secrets/*"}

	_, matched := matchesUserDenylist("/app/data/users.sqlite", patterns)
	if !matched {
		t.Error("should match *.sqlite pattern via basename")
	}

	_, matched = matchesUserDenylist("/app/docs/readme.md", patterns)
	if matched {
		t.Error("should NOT match for a safe .md file")
	}

	// Basename match: *.sqlite should match even on a deep nested path
	_, matched = matchesUserDenylist("/deep/nested/path/db.sqlite", patterns)
	if !matched {
		t.Error("should match *.sqlite pattern via basename on nested path")
	}

	// Empty patterns
	_, matched = matchesUserDenylist("/any/path", nil)
	if matched {
		t.Error("nil patterns should never match")
	}
}

func TestUploadSec_PathsEqualFold(t *testing.T) {
	t.Parallel()

	// On darwin/windows: case-insensitive
	// On linux: case-sensitive
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		if !pathsEqualFold("/Users/Test", "/users/test") {
			t.Error("should be case-insensitive on darwin/windows")
		}
	} else {
		if pathsEqualFold("/Users/Test", "/users/test") {
			t.Error("should be case-sensitive on linux")
		}
	}

	// Always equal when identical
	if !pathsEqualFold("/same/path", "/same/path") {
		t.Error("identical paths should always match")
	}
}

func TestUploadSec_PathHasPrefixFold(t *testing.T) {
	t.Parallel()

	if !pathHasPrefixFold("/uploads/sub/file", "/uploads") {
		t.Error("should match prefix")
	}
	if pathHasPrefixFold("/other/path", "/uploads") {
		t.Error("should not match different prefix")
	}
	if pathHasPrefixFold("/up", "/uploads") {
		t.Error("shorter string should not match longer prefix")
	}

	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		if !pathHasPrefixFold("/Uploads/Sub", "/uploads") {
			t.Error("should be case-insensitive on darwin/windows")
		}
	}
}

// ============================================
// 3. ValidateUploadDir (startup validation)
// ============================================

func TestUploadSec_ValidateUploadDir_Valid(t *testing.T) {
	dir := t.TempDir()
	// Resolve symlinks — macOS /var/folders → /private/var/folders
	dir, _ = filepath.EvalSymlinks(dir)
	sec, err := ValidateUploadDir(dir, nil)
	if err != nil {
		t.Fatalf("ValidateUploadDir(%q) should succeed, got: %v", dir, err)
	}
	if sec.uploadDir == "" {
		t.Error("uploadDir should be set")
	}
}

func TestUploadSec_ValidateUploadDir_Empty(t *testing.T) {
	sec, err := ValidateUploadDir("", nil)
	if err != nil {
		t.Fatalf("empty dir should succeed: %v", err)
	}
	if sec.uploadDir != "" {
		t.Error("uploadDir should be empty for no flag")
	}
}

func TestUploadSec_ValidateUploadDir_Relative(t *testing.T) {
	_, err := ValidateUploadDir("relative/path", nil)
	if err == nil {
		t.Error("relative path should fail")
	}
	if !strings.Contains(err.Error(), "absolute path") {
		t.Errorf("error should mention absolute path, got: %v", err)
	}
}

func TestUploadSec_ValidateUploadDir_NotExists(t *testing.T) {
	_, err := ValidateUploadDir("/nonexistent/dir/12345", nil)
	if err == nil {
		t.Error("nonexistent dir should fail")
	}
}

func TestUploadSec_ValidateUploadDir_IsFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file.txt")
	os.WriteFile(f, []byte("x"), 0o644)

	_, err := ValidateUploadDir(f, nil)
	if err == nil {
		t.Error("file (not directory) should fail")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("error should mention not a directory, got: %v", err)
	}
}

func TestUploadSec_ValidateUploadDir_Symlink(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	os.Symlink(real, link)

	_, err := ValidateUploadDir(link, nil)
	if err == nil {
		t.Error("symlink dir should fail")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error should mention symlink, got: %v", err)
	}
}

func TestUploadSec_ValidateUploadDir_HomeDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	_, err = ValidateUploadDir(home, nil)
	if err == nil {
		t.Error("home directory should be rejected")
	}
	if !strings.Contains(err.Error(), "subdirectory") {
		t.Errorf("error should mention subdirectory, got: %v", err)
	}
}

func TestUploadSec_ValidateUploadDir_SensitiveDir(t *testing.T) {
	home, _ := os.UserHomeDir()
	sshDir := filepath.Join(home, ".ssh")
	// Only test if .ssh exists
	if _, err := os.Stat(sshDir); err != nil {
		t.Skip(".ssh directory does not exist")
	}

	_, err := ValidateUploadDir(sshDir, nil)
	if err == nil {
		t.Error("~/.ssh should be rejected as upload dir")
	}
}

// ============================================
// 4. ValidateFilePath (per-request validation)
// ============================================

func TestUploadSec_ValidateFilePath_ValidFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.mp4")
	os.WriteFile(f, []byte("video data"), 0o644)

	sec := testUploadSecurityWithDir(t, dir)
	result, err := sec.ValidateFilePath(f, true)
	if err != nil {
		t.Fatalf("valid file should pass: %v", err)
	}
	if result.ResolvedPath == "" {
		t.Error("resolved path should be set")
	}
}

func TestUploadSec_ValidateFilePath_RelativePath(t *testing.T) {
	sec := testUploadSecurity(t)
	_, err := sec.ValidateFilePath("../etc/passwd", false)
	if err == nil {
		t.Error("relative path should fail")
	}
	if !strings.Contains(err.Error(), "absolute path") {
		t.Errorf("error should mention absolute path, got: %v", err)
	}
}

func TestUploadSec_ValidateFilePath_DotDotTraversal(t *testing.T) {
	// /tmp/safe/../../../etc/shadow → Clean resolves to /etc/shadow
	sec := testUploadSecurity(t)
	_, err := sec.ValidateFilePath("/tmp/safe/../../../etc/shadow", false)
	if err != nil {
		// The path resolves to /etc/shadow which either doesn't exist or is in denylist.
		// Either way, it should not succeed.
		return
	}
	t.Error("path traversal via .. should be caught")
}

func TestUploadSec_ValidateFilePath_SymlinkToSensitive(t *testing.T) {
	home, _ := os.UserHomeDir()
	sshDir := filepath.Join(home, ".ssh")
	if _, err := os.Stat(sshDir); err != nil {
		t.Skip("~/.ssh does not exist")
	}

	// Create a symlink from temp dir to ~/.ssh
	dir := t.TempDir()
	link := filepath.Join(dir, "innocent.txt")
	target := filepath.Join(sshDir, "known_hosts") // common file in .ssh
	if _, err := os.Stat(target); err != nil {
		t.Skip("~/.ssh/known_hosts does not exist")
	}
	os.Symlink(target, link)

	sec := testUploadSecurityWithDir(t, dir)
	_, err := sec.ValidateFilePath(link, true)
	if err == nil {
		t.Error("symlink to ~/.ssh should be denied")
	}
}

func TestUploadSec_ValidateFilePath_DenylistBlocks(t *testing.T) {
	// Create a real .env file and verify it's blocked
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	os.WriteFile(envFile, []byte("SECRET=abc"), 0o644)

	sec := testUploadSecurityWithDir(t, dir)
	_, err := sec.ValidateFilePath(envFile, true)
	if err == nil {
		t.Error(".env file should be denied by denylist")
	}
	if _, ok := err.(*PathDeniedError); !ok {
		t.Errorf("error should be PathDeniedError, got %T: %v", err, err)
	}
}

func TestUploadSec_ValidateFilePath_KeyFileBlocked(t *testing.T) {
	dir := t.TempDir()
	pemFile := filepath.Join(dir, "server.pem")
	os.WriteFile(pemFile, []byte("-----BEGIN CERTIFICATE-----"), 0o644)

	sec := testUploadSecurityWithDir(t, dir)
	_, err := sec.ValidateFilePath(pemFile, true)
	if err == nil {
		t.Error(".pem file should be denied")
	}
}

// ============================================
// 5. Upload-Dir Enforcement (Stage 1 vs 2-4)
// ============================================

func TestUploadSec_Stage1_WorksWithoutUploadDir(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "video.mp4")
	os.WriteFile(f, []byte("data"), 0o644)

	sec := testUploadSecurityNoDir(t) // no upload-dir
	result, err := sec.ValidateFilePath(f, false)
	if err != nil {
		t.Fatalf("Stage 1 without upload-dir should work: %v", err)
	}
	if result.ResolvedPath == "" {
		t.Error("resolved path should be set")
	}
}

func TestUploadSec_Stage2_RequiresUploadDir(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "video.mp4")
	os.WriteFile(f, []byte("data"), 0o644)

	sec := testUploadSecurityNoDir(t)
	_, err := sec.ValidateFilePath(f, true) // requireUploadDir=true
	if err == nil {
		t.Error("Stage 2+ without upload-dir should fail")
	}
	if _, ok := err.(*UploadDirRequiredError); !ok {
		t.Errorf("should be UploadDirRequiredError, got %T: %v", err, err)
	}
}

func TestUploadSec_OutsideUploadDir_Blocked(t *testing.T) {
	uploadDir := t.TempDir()
	otherDir := t.TempDir()
	f := filepath.Join(otherDir, "video.mp4")
	os.WriteFile(f, []byte("data"), 0o644)

	sec := testUploadSecurityWithDir(t, uploadDir)
	_, err := sec.ValidateFilePath(f, true)
	if err == nil {
		t.Error("file outside upload-dir should be blocked")
	}
}

func TestUploadSec_InsideUploadDir_SubdirAllowed(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub", "deep")
	os.MkdirAll(sub, 0o755)
	f := filepath.Join(sub, "video.mp4")
	os.WriteFile(f, []byte("data"), 0o644)

	sec := testUploadSecurityWithDir(t, dir)
	result, err := sec.ValidateFilePath(f, true)
	if err != nil {
		t.Fatalf("file in subdirectory of upload-dir should be allowed: %v", err)
	}
	if result.ResolvedPath == "" {
		t.Error("resolved path should be set")
	}
}

// ============================================
// 6. Error Message Quality
// ============================================

func TestUploadSec_PathDeniedError_Format(t *testing.T) {
	err := &PathDeniedError{
		Path:      "/home/user/.ssh/id_rsa",
		Pattern:   "~/.ssh",
		UploadDir: "/home/user/Uploads",
	}
	msg := err.Error()
	if !strings.Contains(msg, ".ssh/id_rsa") {
		t.Errorf("error should contain the path, got: %s", msg)
	}
	if !strings.Contains(msg, "~/.ssh") {
		t.Errorf("error should contain the pattern, got: %s", msg)
	}
	if !strings.Contains(msg, "/home/user/Uploads") {
		t.Errorf("error should contain the upload dir, got: %s", msg)
	}
}

func TestUploadSec_UploadDirRequiredError_Format(t *testing.T) {
	err := &UploadDirRequiredError{}
	msg := err.Error()
	if !strings.Contains(msg, "--upload-dir") {
		t.Errorf("error should mention --upload-dir, got: %s", msg)
	}
}

// ============================================
// 7. isWithinDir edge cases
// ============================================

func TestUploadSec_IsWithinDir(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		dir      string
		expected bool
	}{
		{"direct child", "/uploads/file.mp4", "/uploads", true},
		{"nested", "/uploads/sub/file.mp4", "/uploads", true},
		{"same dir", "/uploads", "/uploads", true},
		{"outside", "/other/file.mp4", "/uploads", false},
		{"prefix attack", "/uploads-evil/file.mp4", "/uploads", false},
		{"parent", "/", "/uploads", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isWithinDir(tc.file, tc.dir)
			if got != tc.expected {
				t.Errorf("isWithinDir(%q, %q) = %v, want %v", tc.file, tc.dir, got, tc.expected)
			}
		})
	}
}

// ============================================
// 8. Handler-level integration with security
// ============================================

func TestUploadSec_FileRead_DeniedPath(t *testing.T) {
	// Create a .env file and try to read it via Stage 1
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	os.WriteFile(envFile, []byte("SECRET=abc"), 0o644)

	sec := &UploadSecurity{} // no upload-dir, but denylist still active
	resp := handleFileReadInternal(FileReadRequest{FilePath: envFile}, sec, false)
	if resp.Success {
		t.Error("reading .env file should be denied")
	}
	if !strings.Contains(resp.Error, ".env") {
		t.Errorf("error should mention .env, got: %s", resp.Error)
	}
}

func TestUploadSec_FormSubmit_OutsideUploadDir(t *testing.T) {
	// File exists but is outside the upload-dir
	uploadDir := t.TempDir()
	otherDir := t.TempDir()
	f := filepath.Join(otherDir, "data.txt")
	os.WriteFile(f, []byte("test"), 0o644)

	sec := testUploadSecurityWithDir(t, uploadDir)
	resp := handleFormSubmitInternal(FormSubmitRequest{
		FormAction:    "https://example.com/upload",
		FileInputName: "file",
		FilePath:      f,
	}, sec)

	if resp.Success {
		t.Error("file outside upload-dir should fail for Stage 3")
	}
	if !strings.Contains(resp.Error, "outside") {
		t.Errorf("error should mention outside upload dir, got: %s", resp.Error)
	}
}

// ============================================
// 9. SSRF: unspecified addresses blocked
// ============================================

func TestUploadSec_IsPrivateIP_Unspecified(t *testing.T) {
	unspecified := []string{"0.0.0.0", "0.0.0.1", "::"}
	for _, ipStr := range unspecified {
		t.Run(ipStr, func(t *testing.T) {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				t.Fatalf("failed to parse %q", ipStr)
			}
			if !isPrivateIP(ip) {
				t.Errorf("isPrivateIP(%s) should return true for unspecified address", ipStr)
			}
		})
	}
}

// ============================================
// 10. Hardlink detection
// ============================================

func TestUploadSec_Hardlink_Detected(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original.txt")
	os.WriteFile(original, []byte("secret"), 0o644)

	hardlink := filepath.Join(dir, "hardlink.txt")
	if err := os.Link(original, hardlink); err != nil {
		t.Skip("cannot create hardlinks on this filesystem")
	}

	info, err := os.Stat(hardlink)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkHardlink(info); err == nil {
		t.Error("checkHardlink should detect file with nlink > 1")
	}
}

func TestUploadSec_NoHardlink_Allowed(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "single.txt")
	os.WriteFile(f, []byte("normal"), 0o644)

	info, err := os.Stat(f)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkHardlink(info); err != nil {
		t.Errorf("checkHardlink should allow nlink=1 file: %v", err)
	}
}

// ============================================
// 11. Form field key injection blocked
// ============================================

func TestUploadSec_FormFieldKey_CRLFRejected(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	os.WriteFile(f, []byte("data"), 0o644)

	sec := testUploadSecurityWithDir(t, dir)
	resp := handleFormSubmitInternal(FormSubmitRequest{
		FormAction:    "https://example.com/upload",
		FileInputName: "file",
		FilePath:      f,
		Fields: map[string]string{
			"normal":                "ok",
			"evil\r\nInjected: yes": "attack",
		},
	}, sec)

	if resp.Success {
		t.Error("form submit with CRLF in field key should fail")
	}
	if !strings.Contains(resp.Error, "invalid characters") {
		t.Errorf("error should mention invalid characters, got: %s", resp.Error)
	}
}

// ============================================
// 12. Case-insensitive denylist on macOS
// ============================================

func TestUploadSec_Denylist_CaseInsensitive(t *testing.T) {
	home, _ := os.UserHomeDir()
	if home == "" {
		t.Skip("HOME not set")
	}

	// On macOS (case-insensitive FS), .SSH should match the .ssh denylist
	upperSSH := filepath.Join(home, ".SSH", "id_rsa")
	_, matched := matchesDenylist(upperSSH)

	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		if !matched {
			t.Errorf("matchesDenylist(%q) should match on case-insensitive OS", upperSSH)
		}
	}
	// On Linux (case-sensitive), .SSH != .ssh so no match is expected
}

// ============================================
// 13. DNS fail-closed
// ============================================

func TestUploadSec_SSRF_DNSFailure_Blocked(t *testing.T) {
	err := validateFormActionURL("https://this-domain-definitely-does-not-exist-xyz123.example/upload")
	if err == nil {
		t.Error("validateFormActionURL should reject URL with unresolvable hostname (fail-closed)")
	}
	if !strings.Contains(err.Error(), "DNS") {
		t.Errorf("error should mention DNS failure, got: %v", err)
	}
}

// ============================================
// 14. Root credentials in absolute denylist
// ============================================

func TestUploadSec_Denylist_RootSSH(t *testing.T) {
	_, matched := matchesDenylist("/root/.ssh/id_rsa")
	if !matched {
		t.Error("matchesDenylist should block /root/.ssh/id_rsa (absolute path, no HOME dependency)")
	}
}

func TestUploadSec_Denylist_RootAWS(t *testing.T) {
	_, matched := matchesDenylist("/root/.aws/credentials")
	if !matched {
		t.Error("matchesDenylist should block /root/.aws/credentials")
	}
}
