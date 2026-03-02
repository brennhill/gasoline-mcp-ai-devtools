package upload

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

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
