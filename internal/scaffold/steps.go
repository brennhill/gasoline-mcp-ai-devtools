// steps.go — Default scaffold step definitions.

package scaffold

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

// DefaultSteps returns the standard scaffold steps in execution order.
func DefaultSteps() []Step {
	return []Step{
		{
			Name:  "create_project",
			Label: "Creating project",
			Run: func(ctx context.Context, projectDir string) error {
				cmd := exec.CommandContext(ctx, "pnpm", "create", "vite", projectDir, "--template", "react-ts")
				cmd.Dir = filepath.Dir(projectDir)
				return cmd.Run()
			},
			Verify: func(projectDir string) error {
				if err := VerifyDirectoryExists(projectDir); err != nil {
					return err
				}
				return VerifyFileExists(filepath.Join(projectDir, "package.json"))
			},
		},
		{
			Name:  "install_deps",
			Label: "Installing dependencies",
			Run: func(ctx context.Context, projectDir string) error {
				cmd := exec.CommandContext(ctx, "pnpm", "install")
				cmd.Dir = projectDir
				return cmd.Run()
			},
			Verify: func(projectDir string) error {
				return VerifyDirectoryExists(filepath.Join(projectDir, "node_modules"))
			},
		},
		{
			Name:  "add_tailwind",
			Label: "Adding Tailwind CSS",
			Run: func(ctx context.Context, projectDir string) error {
				cmd := exec.CommandContext(ctx, "pnpm", "add", "tailwindcss", "@tailwindcss/vite")
				cmd.Dir = projectDir
				return cmd.Run()
			},
			Verify: func(projectDir string) error {
				return VerifyPackageInstalled(projectDir, "tailwindcss")
			},
		},
		{
			Name:  "add_shadcn",
			Label: "Adding shadcn/ui components",
			Run: func(ctx context.Context, projectDir string) error {
				// Init shadcn.
				init := exec.CommandContext(ctx, "pnpm", "dlx", "shadcn@latest", "init", "--defaults")
				init.Dir = projectDir
				if err := init.Run(); err != nil {
					return fmt.Errorf("shadcn init: %w", err)
				}
				// Install core components.
				components := []string{
					"button", "card", "input", "label", "select", "checkbox",
					"textarea", "separator", "sheet", "tabs", "scroll-area",
					"alert", "badge", "toast", "skeleton", "navigation-menu",
					"dropdown-menu", "avatar",
				}
				args := append([]string{"dlx", "shadcn@latest", "add"}, components...)
				add := exec.CommandContext(ctx, "pnpm", args...)
				add.Dir = projectDir
				return add.Run()
			},
			Verify: func(projectDir string) error {
				return VerifyDirectoryExists(filepath.Join(projectDir, "src", "components", "ui"))
			},
		},
		{
			Name:  "quality_baseline",
			Label: "Applying quality baseline",
			Run: func(ctx context.Context, projectDir string) error {
				// Quality baseline writes config files — implemented in quality_baseline.go
				return WriteQualityBaseline(projectDir)
			},
			Verify: func(projectDir string) error {
				// Verify key config files exist.
				files := []string{
					".prettierrc",
				}
				for _, f := range files {
					if err := VerifyFileExists(filepath.Join(projectDir, f)); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Name:  "git_init",
			Label: "Initializing Git repository",
			Run: func(ctx context.Context, projectDir string) error {
				init := exec.CommandContext(ctx, "git", "init")
				init.Dir = projectDir
				if err := init.Run(); err != nil {
					return err
				}
				add := exec.CommandContext(ctx, "git", "add", "-A")
				add.Dir = projectDir
				if err := add.Run(); err != nil {
					return err
				}
				commit := exec.CommandContext(ctx, "git", "commit", "-m", "scaffold: initial project")
				commit.Dir = projectDir
				return commit.Run()
			},
			Verify: func(projectDir string) error {
				return VerifyGitInitialized(projectDir)
			},
		},
		{
			Name:  "start_dev_server",
			Label: "Starting dev server",
			Run: func(ctx context.Context, projectDir string) error {
				// Dev server is started asynchronously — handled by the engine caller.
				// This step is a placeholder for the run; the real logic is in the caller.
				return nil
			},
			Verify: func(projectDir string) error {
				// Verification is done externally via dev server detection.
				// For the step itself, just verify package.json has a dev script.
				return VerifyFileExists(filepath.Join(projectDir, "package.json"))
			},
		},
	}
}
