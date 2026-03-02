package upload

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

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
