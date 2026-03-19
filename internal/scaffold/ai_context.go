// ai_context.go — Generates AI context files (CLAUDE.md, bootstrap skill, hooks, .mcp.json).

package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteAIContext generates AI context files based on the wizard configuration.
func WriteAIContext(projectDir string, cfg Config) error {
	files := []struct {
		path    string
		content string
	}{
		{".claude/CLAUDE.md", generateClaudeMD(cfg)},
		{".claude/skills/strum-dev/SKILL.md", generateBootstrapSkill()},
		{".claude/hooks/session-start.js", generateSessionStartHook()},
		{".mcp.json", mcpJSON},
	}

	for _, f := range files {
		path := filepath.Join(projectDir, f.path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("create directory for %s: %w", f.path, err)
		}
		if err := os.WriteFile(path, []byte(f.content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", f.path, err)
		}
	}

	return nil
}

func generateClaudeMD(cfg Config) string {
	return fmt.Sprintf(`# %s

## Project

- **Description:** %s
- **Audience:** %s
- **Current focus:** %s

## Stack

React 19 + Vite + TypeScript (strict) + Tailwind CSS v4 + shadcn/ui + Lucide icons

## Directories

| Path | Contents |
|------|----------|
| src/components/ | UI components (shadcn/ui in src/components/ui/) |
| src/lib/ | Shared utilities |
| src/App.tsx | Main entry point |
| scripts/ | Build and quality scripts |

## Commands

| Command | Purpose |
|---------|---------|
| pnpm dev | Start dev server |
| pnpm build | Production build |
| pnpm lint | Run ESLint |
| pnpm tsc | Type check |
| pnpm test | Run tests |

## Conventions

1. **Tailwind-first** — Use Tailwind classes, never inline styles or CSS modules
2. **shadcn-first** — Use shadcn/ui components when available, don't build custom equivalents
3. **Theme tokens** — Use bg-primary, text-foreground etc. Never hardcode hex colors
4. **Lucide icons** — Use lucide-react for all icons. No inline SVGs
5. **@/ imports** — Always use @/components/... path aliases, never ../
6. **200 LOC max** — Extract when a component exceeds 200 lines
7. **One test per component** — Every .tsx gets a .test.tsx
8. **Responsive from birth** — Every component must work at 375px width
9. **Accessible by default** — Labels on inputs, alt on images, focus states on interactive elements
`, cfg.Name, cfg.Description, cfg.Audience, cfg.FirstFeature)
}

func generateBootstrapSkill() string {
	return `# Strum Dev Skill

## Overview

Bootstrap skill for Strum-scaffolded projects. Teaches the AI agent project conventions
and how to use annotation mode for visual editing.

## Anti-Slop Invariants

These rules are enforced by pre-commit hooks. Do not rationalize around them.

| Rule | Enforcement | Blocked Rationalization |
|------|-------------|----------------------|
| **Tailwind-first** | No inline style={}. No CSS modules. | "Just a quick inline style" |
| **shadcn-first** | Use shadcn components when available. | "I'll build a custom button" |
| **Theme tokens only** | Use bg-primary, text-foreground. No hex. | "I'll use the hex directly" |
| **Lucide icons only** | Use lucide-react. No inline SVGs. | "Just one inline SVG" |
| **200 LOC max** | Extract when exceeding 200 lines. | "Keep it all in one file" |
| **No barrel exports** | Import from component files directly. | "Add an index.ts" |
| **@/ imports** | Use path aliases. Never ../../../. | "Relative path is fine" |
| **One test per component** | Every .tsx gets a .test.tsx. | "Add tests later" |
| **Responsive from birth** | Must not break at 375px width. | "Handle mobile later" |
| **Accessible by default** | Labels, alt text, focus states. | "Accessibility later" |

## Annotation Mode

Use Gasoline's annotation mode to visually inspect and edit components:
1. Click any element in the browser to see its source code
2. Edit Tailwind classes for instant visual feedback via HMR
3. Components in src/components/ui/ are editable shadcn source files

## MCP Tools

- observe: Read browser state (console, network, screenshot)
- interact: Navigate, click, type, resize
- analyze: Accessibility audit, security scan, annotations
- generate: CSP headers, test plans
- configure: Health checks, settings
`
}

func generateSessionStartHook() string {
	return `// session-start.js — Bootstrap context injection hook.
// Injects the bootstrap skill as additional context on session start,
// /clear, and context compaction so the AI always knows project conventions.

const fs = require('fs')
const path = require('path')

const skillPath = path.join(__dirname, '..', 'skills', 'strum-dev', 'SKILL.md')

module.exports = {
  event: 'SessionStart',
  handler: () => {
    try {
      const skill = fs.readFileSync(skillPath, 'utf-8')
      return {
        additionalContext: skill,
      }
    } catch (err) {
      // Skill file missing — don't block session start.
      return {}
    }
  },
}
`
}

const mcpJSON = `{
  "mcpServers": {
    "gasoline": {
      "command": "gasoline-mcp",
      "args": [],
      "env": {}
    }
  }
}
`
