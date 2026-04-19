//go:build windows

// runner_windows.go — Self-update is not supported on Windows yet.
// The install script is bash-only; Windows users must re-run the installer.

package upgrade

// Spawn returns ErrUnsupportedPlatform on Windows.
func Spawn(_ string) error {
	return ErrUnsupportedPlatform
}
