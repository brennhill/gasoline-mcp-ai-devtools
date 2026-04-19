//go:build !windows

// runner_unix.go — Detached install-script spawn for Unix platforms.
// The script pkills the running daemon before writing the new binary, so the
// child must not share the daemon's process group or the TERM propagates back.

package upgrade

import (
	"os"
	"os/exec"
	"syscall"
)

// Spawn launches the install script in a new session with fully detached stdio
// and returns as soon as the child process is started. It is safe for the
// daemon to exit immediately after — the child survives parent death because
// Setsid gives it a new session and process group.
func Spawn(pinnedURL string) error {
	name, args, err := buildInstallCmd(pinnedURL)
	if err != nil {
		return err
	}
	cmd := exec.Command(name, args...) // #nosec G204 -- URL is validated by buildInstallCmd
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Env = append(os.Environ(), "KABOOM_SELF_UPDATE=1")
	if err := cmd.Start(); err != nil {
		return err
	}
	// Release the Process struct so we don't retain a wait channel for a
	// child we never intend to reap.
	_ = cmd.Process.Release()
	return nil
}
