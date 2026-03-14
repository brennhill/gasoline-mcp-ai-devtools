// Purpose: Installer terminal output formatting (panels, checklists, status messages).
// Why: Isolates presentation from install logic and config I/O.

package main

import "fmt"

func manualExtensionSetupChecklist(extDir string) []string {
	return []string{
		"BROWSER EXTENSION (MANUAL STEP REQUIRED):",
		"   The installer staged extension files, but it cannot click browser UI controls for you.",
		"   1) Open chrome://extensions (or brave://extensions)",
		"   2) Enable Developer mode",
		"   3) Click Load unpacked and select:",
		fmt.Sprintf("      %s", extDir),
		"   4) Pin Gasoline in the browser toolbar (recommended)",
		"   5) Open the Gasoline popup and click Track This Tab",
	}
}

func printManualExtensionSetupChecklist(extDir string) {
	lines := manualExtensionSetupChecklist(extDir)
	if len(lines) == 0 {
		return
	}
	stderrf("\033[1;33m%s\033[0m\n", lines[0])
	for _, line := range lines[1:] {
		stderrf("%s\n", line)
	}
}

func printInstallerPanel(title string, lines []string) {
	const border = "+----------------------------------------------------------+"
	stderrf("\033[1;36m%s\033[0m\n", border)
	stderrf("\033[1;36m| \033[1m%-56s\033[1;36m |\033[0m\n", title)
	stderrf("\033[1;36m%s\033[0m\n", border)
	for _, line := range lines {
		stderrf("\033[1;36m|\033[0m %-58s \033[1;36m|\033[0m\n", line)
	}
	stderrf("\033[1;36m%s\033[0m\n", border)
}

func printInstallSuccess(exe, extDir string) {
	stderrf("\n\033[1;32m✅ GASOLINE INSTALLED & RUNNING!\033[0m\n")
	printInstallerPanel("INSTALL SUMMARY", []string{
		"Gasoline server started in background on port 7890.",
		"MCP clients are configured with direct binary path (no npx).",
		fmt.Sprintf("Binary path: %s", exe),
	})
	stderrf("\n")
	printManualExtensionSetupChecklist(extDir)
	stderrf("\033[1;33mREADY TO COOK:\033[0m\n")
	stderrf("   The Gasoline server is active on port 7890.\n")
	stderrf("   Your AI tool (Claude, Cursor, etc.) is now configured.\n")
	stderrf("\033[1;36m+----------------------------------------------------------+\033[0m\n")
}
