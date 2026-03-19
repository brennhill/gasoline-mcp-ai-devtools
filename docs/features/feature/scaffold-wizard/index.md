# Scaffold Wizard

status: spec
last_reviewed: 2026-03-19

## Overview

A browser-based scaffold wizard that lets users create a new web app from inside Strum. The user answers a few guided questions, the scaffold builds locally, the dev server starts, and the browser navigates to the running app with Gasoline already connected and monitoring.

Goal: go from "I want to build X" to editing in the browser with annotation mode in under 60 seconds.

## Design Influences

Two frameworks informed the design:

**Superpowers** (github.com/obra/superpowers) — Socratic refinement and anti-rationalization. The wizard doesn't accept vague answers; it guides users through progressive follow-ups to produce a clear spec before any code runs. Post-scaffold, a bootstrap skill orients the AI agent to the project conventions.

**Get Shit Done** (github.com/gsd-build/get-shit-done) — Defense-in-depth verification. Every scaffold step has an automated check. Goal-backward verification confirms the user can actually edit and see changes, not just that commands succeeded. Four-level artifact checks (Exists -> Substantive -> Wired -> Functional) catch stubs and broken wiring.

## User Flow

### Conversational Wizard (not a form)

The wizard is a short conversation, not a wall of inputs. Each step builds on the previous answer.

**Step 1: "What are you building?"**
Free text. Examples shown as inspiration: "a todo app", "a dashboard for my sales data", "a landing page for my startup".

**Step 2: "Who is it for?"**
Three choices: "Just me" / "My team" / "Public users"
This shapes the scaffold — auth scaffolding for team/public, simpler for personal.

**Step 3: "What's the most important first feature?"**
Free text, one thing. Forces the user to prioritize before code exists. This becomes the seed content in App.tsx and the first task in the generated CLAUDE.md.

**Step 4: Project name**
Auto-generated from Step 1 (slugified). Editable, but most users won't touch it.

Then: "Create" button. That's it — 4 interactions, ~30 seconds.

### Build Phase

1. Scaffold runs in background (progress streams to wizard UI)
2. Each step shows status: spinner -> checkmark or error
3. Dev server starts, port detected
4. Browser navigates to running app
5. Gasoline auto-tracks the new tab
6. Wizard shows "Ready — start editing"

## Opinionated Stack

We choose for them. The point is speed to editing, not framework selection.

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Framework | React + Vite | Largest ecosystem, best AI training data, fastest HMR |
| Language | TypeScript (strict) | Better AI completions, catches errors early |
| Styling | Tailwind CSS v4 | Utility-first, annotation-mode friendly — class edits = instant visual feedback via HMR |
| Components | shadcn/ui | Copy-paste into user source tree (not node_modules), excellent defaults, annotation mode can locate and edit actual files |
| Package manager | pnpm | Fast, disk-efficient, deterministic lockfile |
| Directory | `~/strum-projects/<slug>` | Discoverable, no decision paralysis |

### Why This Stack for Annotation Mode

Annotation mode lets users click elements and edit them visually. This stack maximizes that experience:

- **Tailwind** — changing `bg-blue-500` to `bg-red-500` is a single class attribute edit, HMR reflects it instantly
- **shadcn/ui** — components live in the user's `src/components/ui/` (not behind `node_modules`), so annotation mode can map DOM elements to editable source files
- **Vite** — sub-100ms HMR means edits feel instant, no perceptible delay
- **React** — component boundaries map cleanly to DOM elements, making annotation-to-source-file mapping reliable

## Architecture

```
+----------------------------------+
|  Browser: localhost:7890/launch   |
|  +----------------------------+  |
|  |  Conversational Wizard     |  |
|  |  4 guided steps + Create   |  |
|  +-------------+--------------+  |
+-----------------|----------------+
                  | POST /api/scaffold
                  v
+----------------------------------+
|  Gasoline Daemon (port 7890)     |
|  +----------------------------+  |
|  |  Scaffold Handler          |  |
|  |  1. pnpm create vite       |  |
|  |  2. Install tailwind/shadcn|  |
|  |  3. Apply quality baseline |  |
|  |  4. Verify each step       |  |
|  |  5. pnpm dev               |  |
|  |  6. Verify dev server      |  |
|  +-------------+--------------+  |
+-----------------|----------------+
                  | interact(navigate)
                  v
+----------------------------------+
|  Browser: localhost:5173          |
|  (new app, Gasoline tracking,    |
|   annotation mode ready)         |
+----------------------------------+
```

## Components to Build

### 1. Wizard Landing Page

**Route:** `GET /launch`
**Served by:** Gasoline daemon HTTP server (port 7890)
**Content:** Static HTML/CSS/JS — minimal, no framework needed for the wizard itself

UI design:
- Conversational flow, one question at a time (not a form)
- Each step slides in after the previous is answered
- Dark theme matching Gasoline popup aesthetic
- Progress bar showing 4 steps
- "Create" button appears after Step 4
- Build progress area replaces the wizard once creation starts

### 2. Scaffold API Endpoint

**Route:** `POST /api/scaffold`
**Request body:**
```json
{
  "description": "a todo app with drag and drop",
  "audience": "just_me",
  "first_feature": "drag and drop reordering",
  "name": "todo-app"
}
```

**Response:** Server-Sent Events (SSE) stream of progress updates:
```
event: step
data: {"step": "create_project", "status": "running", "label": "Creating project..."}

event: step
data: {"step": "create_project", "status": "done", "label": "Project created"}

event: step
data: {"step": "install_deps", "status": "running", "label": "Installing dependencies..."}

event: step
data: {"step": "verify_install", "status": "done", "label": "Dependencies verified"}

event: complete
data: {"url": "http://localhost:5173", "project_dir": "/Users/you/strum-projects/todo-app"}

event: error
data: {"step": "install_deps", "error": "pnpm not found", "recovery": "Install pnpm: npm install -g pnpm"}
```

### 3. Scaffold Engine

Executes in sequence inside a PTY session. Each step has a verification gate.

| Step | Command | Verification |
|------|---------|-------------|
| 1. Create project | `pnpm create vite <dir> --template react-ts` | Directory exists, `package.json` present |
| 2. Install deps | `pnpm install` | `node_modules/` exists, exit code 0 |
| 3. Add Tailwind | `pnpm add tailwindcss @tailwindcss/vite` | Package in `node_modules/` |
| 4. Add shadcn | `pnpm dlx shadcn@latest init --defaults` | `src/components/ui/` exists |
| 5. Install shadcn components | `pnpm dlx shadcn@latest add button card` | Component files exist |
| 6. Apply quality baseline | Write config files, replace App.tsx | Files exist, `tsc --noEmit` passes |
| 7. Git init | `git init && git add -A && git commit` | `.git/` exists |
| 8. Start dev server | `pnpm dev` | HTTP 200 on detected port |

If any verification fails, the step is retried once. If still failing, the error streams to the wizard UI with a recovery suggestion.

### 4. Quality Baseline

Applied after scaffold, before dev server start:

**tsconfig.json** — enforce strict mode:
- `strict: true`
- `noUncheckedIndexedAccess: true`
- `noUnusedLocals: true`
- `noUnusedParameters: true`

**ESLint** — pre-configured with:
- `@eslint/js` recommended
- `typescript-eslint` strict
- `eslint-plugin-react-hooks`

**Prettier** — consistent formatting:
- Single quotes, no semicolons, 2-space indent, trailing commas

**Example component** — replace default `App.tsx` with a shadcn-based starter:
- Uses shadcn Button and Card components
- Uses Tailwind classes throughout
- Shows the project description from the wizard as hero text
- Shows the "first feature" as a placeholder section
- Gives the user something meaningful to start editing in annotation mode

**Git** — `git init` + `.gitignore` + initial commit ("scaffold: <project name>")

### 5. AI Context Generation (Bootstrap Skill)

Generated into the project so any AI agent immediately understands it.

**`.claude/CLAUDE.md`** — project context for Claude Code:
- Project name and description (from wizard Step 1)
- Audience (from Step 2)
- First feature / current focus (from Step 3)
- Stack summary (React 19, Vite, TypeScript strict, Tailwind v4, shadcn/ui)
- Key directories and what lives where
- Dev commands (`pnpm dev`, `pnpm build`, `pnpm lint`, `pnpm tsc`)
- Conventions: component naming, file organization, Tailwind-first styling

**`.claude/skills/strum-dev/SKILL.md`** — bootstrap skill (Superpowers pattern):
- Teaches the agent how to use annotation mode for visual editing
- References the gasoline MCP tools for browser interaction
- Enforces Tailwind-first styling (no custom CSS unless Tailwind can't do it)
- Enforces shadcn component reuse before creating new components
- Includes anti-rationalization table for common shortcuts agents take

**`.mcp.json`** — pre-wires Gasoline as an MCP server so tools are available immediately.

### 6. Dev Server Detection

After `pnpm dev` starts:
- Parse stdout for Vite's "Local: http://localhost:XXXX" message (regex: `Local:\s+http://localhost:(\d+)`)
- Fallback: poll ports 5173-5180 for HTTP 200
- Timeout after 30 seconds with error

### 7. Auto-Navigate and Connect

Once dev server is ready:
1. Call `interact(what='navigate', url='http://localhost:<port>')` via internal API
2. Gasoline auto-tracks the new tab
3. Wizard UI transitions to "Ready" state with:
   - Link to the running app
   - Project directory path
   - "Open in editor" suggestion (VS Code / Cursor)
   - Quick tips for annotation mode
4. Annotation mode is immediately available on the new page

### 8. Goal-Backward Verification (GSD Pattern)

After the full scaffold completes, run a final verification pass. Don't check "did commands succeed" — check "can the user achieve the goal?"

**The goal:** User can click an element in the browser, edit it, and see the change.

**Verification chain (work backward from goal):**

1. **Functional:** Edit `App.tsx` (change a Tailwind class), verify HMR updates the browser within 2 seconds
2. **Wired:** `App.tsx` imports shadcn components, Tailwind classes resolve, Vite config includes Tailwind plugin
3. **Substantive:** `App.tsx` is not Vite boilerplate — it contains project-specific content from the wizard
4. **Exists:** All expected files present (`App.tsx`, `tailwind.config.ts`, `components/ui/button.tsx`, `.claude/CLAUDE.md`)

If any level fails, report which level broke and what's missing.

## Prerequisites

- `node` >= 18 on PATH
- `pnpm` on PATH (wizard offers to install if missing: `npm install -g pnpm`)
- Gasoline daemon running (it serves the wizard — this is always true if the user opened `/launch`)
- Chrome with Gasoline extension installed (for annotation mode; scaffold works without it)

The wizard checks these before showing the "Create" button and shows clear, actionable errors.

## Open Questions

- **Error recovery**: if scaffold fails mid-way, clean up partial directory or leave for debugging?
- **Project gallery**: should `/launch` also list existing strum-projects for re-opening?
- **Port conflicts**: if 5173 is taken, Vite auto-picks another — need to parse the actual port from stdout
- **Template variants**: start with one template (React + shadcn), add more later? (e.g. landing page, dashboard, API client)
- **AI agent integration**: should the wizard offer to open Claude Code / Cursor in the project directory after scaffold?
- **Offline support**: can we bundle Vite templates in the binary for air-gapped environments?
- **Project persistence**: should `/launch` remember recent projects and offer to reopen dev servers?

## Success Criteria

- Cold start (no project exists) to editable app in browser: < 60 seconds on broadband
- Annotation mode works immediately on the scaffolded app
- HMR works — editing a Tailwind class in source reflects in browser within 200ms
- All generated code passes `tsc --noEmit` and `eslint` with zero errors
- User never needs to touch a terminal — everything happens from the wizard
- Goal-backward verification passes on every scaffold run
- CLAUDE.md is accurate and useful — an AI agent can start working on the project without additional context
- Bootstrap skill prevents common AI mistakes (custom CSS over Tailwind, creating components that shadcn already provides)
