// verify.go — Verification gate functions for scaffold steps.

package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
)

// VerifyDirectoryExists checks that a directory exists at the given path.
func VerifyDirectoryExists(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("directory does not exist: %s", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}
	return nil
}

// VerifyFileExists checks that a file exists at the given path.
func VerifyFileExists(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("file does not exist: %s", path)
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", path)
	}
	return nil
}

// VerifyPackageInstalled checks that a package exists in node_modules.
func VerifyPackageInstalled(projectDir, pkg string) error {
	pkgDir := filepath.Join(projectDir, "node_modules", pkg)
	return VerifyDirectoryExists(pkgDir)
}

// VerifyGitInitialized checks that .git exists in the project directory.
func VerifyGitInitialized(projectDir string) error {
	return VerifyDirectoryExists(filepath.Join(projectDir, ".git"))
}
