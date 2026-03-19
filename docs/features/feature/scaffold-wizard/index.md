# Scaffold Wizard

status: spec
last_reviewed: 2026-03-19

## Overview

A browser-based scaffold wizard that lets users create a new web app from inside Strum. Two phases: a fast automated scaffold builds the empty project and starts the dev server, then an AI agent takes over and **live-composes the first real UI in the browser** — the user watches components appear, layouts form, and content populate in real-time.

Goal: go from "I want to build X" to a meaningful, editable app in the browser — not boilerplate, a real first screen — in under 90 seconds.

## Design Influences

**Pencil** (pencil.dev) — Live composition. Pencil lets you watch Claude build a design in real-time inside the app via atomic batch operations, progressive rendering, placeholder indicators, and screenshot verification loops. We apply this same pattern: the user watches their app being built in the browser, not in a terminal.

**Superpowers** (github.com/obra/superpowers) — Socratic refinement and anti-rationalization. The wizard guides users through progressive follow-ups before any code runs. Post-scaffold, a bootstrap skill orients the AI agent to project conventions.

**Get Shit Done** (github.com/gsd-build/get-shit-done) — Defense-in-depth verification. Every step has an automated check. Goal-backward verification confirms the user can actually edit and see changes. Four-level artifact checks (Exists -> Substantive -> Wired -> Functional) catch stubs and broken wiring.

## User Flow

### Conversational Wizard (not a form)

The wizard is a guided conversation, not a wall of inputs. Each step builds on the previous answer. The wizard handles both the creative decisions (what to build) and the infrastructure setup (where to save, how to deploy), explaining *why* at each step so the user understands what's being set up and isn't surprised later.

#### Part 1: What Are You Building? (~30 seconds)

**Step 1: "What are you building?"**
Free text. Examples shown as inspiration: "a todo app", "a dashboard for my sales data", "a landing page for my startup".

**Step 2: "Who is it for?"**
Three choices: "Just me" / "My team" / "Public users"
This shapes the scaffold — auth scaffolding for team/public, simpler for personal.

**Step 3: "What's the most important first feature?"**
Free text, one thing. Forces the user to prioritize before code exists. This becomes the seed for the AI composition phase.

**Step 4: Project name**
Auto-generated from Step 1 (slugified). Editable, but most users won't touch it.

#### Part 2: Set Up Your Safety Net (~30 seconds)

After the creative questions, the wizard explains the infrastructure it's about to set up. This isn't a configuration screen — it's a brief explainer with one-click setup for each service. The user can skip any step, but the wizard tells them what they'd be missing.

**Step 5: "Let's back up your code"**
*"We'll save your project to GitHub so you never lose your work. It's free and takes 10 seconds."*

- If `gh` is installed and authenticated: show green checkmark, "Connected as @username"
- If `gh` is installed but not authenticated: "Sign in to GitHub" button → runs `gh auth login` in embedded terminal
- If `gh` not installed: "Install GitHub CLI" link + brief instructions
- Skip option: "I'll do this later" (small text, not prominent)

On completion: creates private repo, pushes initial code. The wizard shows: *"Your code is backed up. Every change you make will be auto-saved."*

**Step 6: "Where should we deploy?"**
*"Pick where your app goes live. All are free to start. One click to deploy anytime."*

Three cards showing deploy platforms (whichever has the best rev share deal is marked "Recommended"):
- Vercel — "Best for React apps. Instant preview URLs on every change."
- Cloudflare — "Fastest global network. Generous free tier."
- Netlify — "Simple and reliable. Great for getting started."

Each card has a "Connect" button that runs the platform's auth flow. Skip option available.

**Step 7: "Keep your code secure"**
*"We'll scan for vulnerabilities before every deploy. Connect a scanner for continuous monitoring."*

Cards for scanner partners (best rev share deal marked "Recommended"):
- Codacy — "Code quality + security in one. Auto-reviews every change."
- Snyk — "Best at catching vulnerable dependencies."
- SonarCloud — "Deep analysis. Free for open source."

Each has a "Connect" button. Skip option available.

**Step 8: "Track errors in production"**
*"Your app catches errors automatically during development. Want to know about errors after you deploy?"*

Only shown if a deploy platform was selected in Step 6. Cards:
- Sentry — "Industry standard. See every error with full context."
- BetterStack — "Errors + uptime + logs in one dashboard."

Each has a "Connect" button. Skip option available.

**Then: "Create" button.**

The wizard summarizes what's about to happen:
```
Ready to create "todo-app":
  ✓ React + TypeScript + Tailwind + shadcn
  ✓ Backed up to GitHub (private)
  ✓ Deploys to Vercel
  ✓ Security scans by Codacy
  ✓ Error tracking by Sentry
  → Click Create to start building
```

Total wizard time: ~60-90 seconds (30s creative + 30-60s infrastructure). Every infrastructure step is skippable but explained. Everything after this is automated.

### Phase 1: Automated Scaffold (~20 seconds)

Fast, deterministic, no AI needed. Sets up the full project with everything the wizard configured:

1. Create Vite + React + TypeScript project
2. Install dependencies (pnpm)
3. Add Tailwind CSS v4 + shadcn/ui + lucide-react
4. Apply quality baseline (tsconfig strict, eslint, prettier, vitest)
5. Generate CLAUDE.md, bootstrap skill, hooks, deploy configs
6. Git init + initial commit
7. Push to GitHub (if connected in Step 5)
8. Connect deploy platform (if selected in Step 6)
9. Connect security scanner (if selected in Step 7)
10. Start dev server
11. Detect ready port, navigate browser to it
12. Gasoline auto-tracks the new tab

Progress streams to wizard UI via WebSocket. Each step shows: spinner → checkmark.

### Phase 2: AI Live Composition (the Pencil Experience)

This is the magic. Once the dev server is running and showing a blank app, an AI agent takes over and builds the first real UI — live, in the browser, while the user watches.

The agent uses Gasoline's own tools (`interact`, `observe`, `analyze`) to:

1. **Read the user's description and first feature** from the wizard context
2. **Plan the layout** — header, main content area, feature section, footer
3. **Write components incrementally** — each file write triggers Vite HMR, the browser updates instantly
4. **Verify each step visually** — `observe(what='screenshot')` after each component, check for layout issues
5. **Correct and iterate** — if something looks wrong, fix it and re-verify

**What the user sees:**

```
Phase 1: Setting up project...     ✓ (15s)
Phase 2: Building your app...

  [Browser shows the app updating live]

  ✓ Created layout structure          (header, sidebar, main)
  ✓ Added navigation component        (logo, menu items)
  → Building "drag and drop" feature   (writing TodoList.tsx...)
  ○ Adding sample content
  ○ Final polish
```

The browser is the canvas. Each component write → HMR update → screenshot verify cycle takes 2-5 seconds. A full first screen builds out over 30-60 seconds, with visible progress at every step — exactly like watching Pencil compose a design, but with a real running app.

### How It Maps to Pencil's Mechanisms

| Pencil | Strum Phase 2 |
|--------|---------------|
| `batch_design` (25 atomic ops) | Write component file → HMR renders it instantly |
| WASM re-render after each batch | Vite HMR updates browser after each file write |
| `placeholder: true` frames | Skeleton/placeholder components rendered first, then populated |
| `get_screenshot` → verify → correct | `observe(what='screenshot')` → check layout → fix if needed |
| Fine-grained tool streaming | File writes stream to disk, HMR picks up partial saves |
| Multiple small batches = progressive | One component at a time, each immediately visible |

### The Composition Loop

The AI agent runs this loop for each component:

```
1. Write file (e.g., src/components/TodoList.tsx)
   → Vite HMR fires, browser updates
2. Screenshot the page
   → observe(what='screenshot')
3. Verify layout
   → No overlapping elements? Text visible? Responsive?
4. If issues: fix and goto 2
5. If good: next component
```

This is design-time TDD applied to real code. Build, verify, fix — the user sees every iteration.

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

### Why This Stack for Live Composition + Annotation Mode

- **Tailwind** — changing `bg-blue-500` to `bg-red-500` is a single class attribute edit, HMR reflects it instantly. The AI agent can iterate on styling in real-time.
- **shadcn/ui** — components live in `src/components/ui/` (not `node_modules`), so both the AI agent and annotation mode can locate and edit actual source files.
- **Vite** — sub-100ms HMR means each component write is instantly visible. This is what makes the live composition feel real-time.
- **React** — component boundaries map cleanly to DOM elements, making both AI composition and annotation-to-source-file mapping reliable.

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
+----------------------------------+     PHASE 1: Automated (~15s)
|  Gasoline Daemon (port 7890)     |
|  +----------------------------+  |
|  |  Scaffold Handler          |  |
|  |  1. pnpm create vite       |  |
|  |  2. Install tailwind/shadcn|  |
|  |  3. Quality baseline       |  |
|  |  4. Generate hooks + skill |  |
|  |  5. pnpm dev               |  |
|  +-------------+--------------+  |
+-----------------|----------------+
                  | interact(navigate)
                  v
+----------------------------------+     PHASE 2: AI Live Composition (~60s)
|  Browser: localhost:5173          |
|  +----------------------------+  |
|  |  AI agent builds UI live:  |  |
|  |  write component → HMR    |  |
|  |  → screenshot → verify    |  |
|  |  → next component         |  |
|  |  (user watches it happen)  |  |
|  +----------------------------+  |
|  Gasoline tracking + annotation  |
+----------------------------------+

WebSocket (port 7891):
+----------------------------------+
| Multiplexed channels:            |
| - scaffold: Phase 1 progress     |
| - compose:  Phase 2 AI progress  |
| - terminal: raw PTY output       |
| - wizard:   user interactions    |
+----------------------------------+

Generated project hooks:
+----------------------------------+
|  .claude/hooks/                  |
|  - session-start.js  (bootstrap) |
|  - context-monitor.js (warnings) |
|  - statusline.js     (dashboard) |
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
- Phase 1 progress replaces wizard questions
- Phase 2 shows split view: browser preview + composition progress log

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

**Response:** HTTP 202 Accepted with a channel ID. All progress streams over the WebSocket on port 7891:
```json
{"status": "accepted", "channel": "scaffold-todo-app-1710884400"}
```

### 3. Phase 1: Scaffold Engine

Executes in sequence inside a PTY session. Each step has a verification gate.

| Step | Command | Verification |
|------|---------|-------------|
| 1. Create project | `pnpm create vite <dir> --template react-ts` | Directory exists, `package.json` present |
| 2. Install deps | `pnpm install` | `node_modules/` exists, exit code 0 |
| 3. Add Tailwind | `pnpm add tailwindcss @tailwindcss/vite` | Package in `node_modules/` |
| 4. Add shadcn | `pnpm dlx shadcn@latest init --defaults` | `src/components/ui/` exists |
| 5. Install shadcn components | `pnpm dlx shadcn@latest add button card input label select checkbox textarea separator sheet tabs scroll-area alert badge toast skeleton navigation-menu dropdown-menu avatar` | All component files exist in `src/components/ui/` |
| 5b. Install icons | `pnpm add lucide-react` | Package in `node_modules/` |
| 6. Quality baseline | Write config files, path aliases, theme, minimal App.tsx shell | `tsc --noEmit` passes |
| 7. Generate AI context | Write CLAUDE.md, bootstrap skill, hooks, .mcp.json | Files exist |
| 8. Git init | `git init && git add -A && git commit` | `.git/` exists |
| 9. Start dev server | `pnpm dev` | HTTP 200 on detected port |

If any verification fails, retry once. If still failing, stream error with recovery suggestion.

### 4. Phase 2: AI Composition Engine

After Phase 1 completes and the browser shows the running app, the AI agent takes over. It receives:

**Input context:**
- User's description ("a todo app with drag and drop")
- Audience ("just me")
- First feature ("drag and drop reordering")
- Available shadcn components (from the install)
- Project directory path

**Composition strategy:**

The agent works **outside-in**: layout shell first, then sections, then detail.

1. **Layout structure** — write `src/components/Layout.tsx` with header, main area, optional sidebar. HMR fires, browser shows the skeleton.

2. **Navigation** — write `src/components/Header.tsx` with project name, nav items. Screenshot, verify header renders.

3. **Primary feature** — build the first feature component(s) based on Step 3 of the wizard. For "drag and drop reordering": write `src/components/TodoList.tsx`, `src/components/TodoItem.tsx`. Each file write → HMR → screenshot → verify.

4. **Sample content** — populate with realistic placeholder data so the app looks alive, not empty.

5. **Responsive check** — resize viewport to mobile width (375px) via `interact(what='resize', width=375)` + screenshot. Fix any components that break at mobile. Resize back.

6. **Accessibility check** — run `analyze(what='accessibility')`. Fix WCAG violations (missing alt text, contrast issues, missing labels) before declaring done.

7. **Final commit** — `git add -A && git commit -m "feat: initial UI composition"`

**Composition WebSocket messages:**
```json
{"channel": "compose", "phase": "layout", "status": "writing", "file": "src/components/Layout.tsx"}
{"channel": "compose", "phase": "layout", "status": "hmr_update"}
{"channel": "compose", "phase": "layout", "status": "screenshot", "result": "pass"}
{"channel": "compose", "phase": "navigation", "status": "writing", "file": "src/components/Header.tsx"}
{"channel": "compose", "phase": "feature", "status": "writing", "file": "src/components/TodoList.tsx"}
{"channel": "compose", "phase": "feature", "status": "screenshot", "result": "fix_needed", "issue": "text not visible"}
{"channel": "compose", "phase": "feature", "status": "fixing", "file": "src/components/TodoList.tsx"}
{"channel": "compose", "phase": "complete", "components_created": 5, "duration_s": 45}
```

### 5. Quality Baseline (Phase 1)

Applied during Phase 1, before dev server start.

#### Compiler & Linting

**tsconfig.json** — strict mode:
- `strict: true`
- `noUncheckedIndexedAccess: true`
- `noUnusedLocals: true`
- `noUnusedParameters: true`
- Path aliases: `"@/*": ["./src/*"]` (prevents `../../../../` import hell)

**vite.config.ts** — matching path alias via `resolve.alias`

**ESLint** — `@eslint/js` recommended + `typescript-eslint` strict + `eslint-plugin-react-hooks`

**Prettier** — single quotes, no semicolons, 2-space indent, trailing commas

#### Curated Component Library

shadcn/ui components pre-installed to cover 90% of first-screen needs. The AI agent in Phase 2 composes from these instead of writing raw divs.

| Category | Components |
|----------|-----------|
| Layout | `card`, `separator`, `sheet`, `tabs`, `scroll-area` |
| Forms | `input`, `button`, `label`, `select`, `checkbox`, `textarea` |
| Feedback | `alert`, `badge`, `toast`, `skeleton` |
| Navigation | `navigation-menu`, `dropdown-menu`, `avatar` |

**Icons:** `lucide-react` pre-installed (the icon library shadcn uses). Bootstrap skill enforces: use Lucide, don't inline SVGs, don't add another icon library.

#### Tailwind Theme with Design Tokens

The scaffold generates a Tailwind theme using CSS variables for brand colors, not hardcoded hex values:

```css
/* src/index.css */
@theme {
  --color-primary: #2563eb;
  --color-secondary: #64748b;
  --color-accent: #f59e0b;
  --color-background: #ffffff;
  --color-foreground: #0f172a;
}
```

Bootstrap skill enforces: use `bg-primary`, `text-foreground`, etc. — never `bg-[#2563eb]` or inline hex colors.

#### Testing

**Vitest** pre-configured with:
- `vitest.config.ts` wired with `happy-dom`, `@/` path aliases, coverage reporter
- `pnpm test` runs all tests
- One example test generated to prove the setup works
- Phase 2 AI agent writes one test per composed component

#### Minimal App.tsx Shell

Just enough for the dev server to show something. Phase 2 replaces this with real content.

#### Git

`git init` + `.gitignore` + initial commit ("scaffold: <project name>")

### 6. AI Context Generation (Bootstrap Skill)

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
- Anti-rationalization table for common AI shortcuts

**Bootstrap skill enforced rules (anti-slop invariants):**

| Rule | Enforcement | Rationalization it blocks |
|------|-------------|--------------------------|
| **Tailwind-first** | No inline `style={}`. No CSS modules. Use Tailwind classes. | "I'll just add a quick inline style for this one thing" |
| **shadcn-first** | If shadcn has a component, use it. Don't create custom equivalents. | "I'll build a quick custom button, it's simpler" |
| **Theme tokens only** | Use `bg-primary`, `text-foreground`. Never `bg-[#2563eb]` or hardcoded hex. | "I'll just use the hex value directly, it's faster" |
| **Lucide icons only** | Use `lucide-react`. No inline SVGs. No other icon libraries. | "I'll paste this SVG inline, it's just one icon" |
| **200 LOC max per file** | Extract when a component exceeds 200 lines. | "I'll keep it all in one file for now, it's easier to follow" |
| **No barrel exports** | Import directly from component files. No `index.ts` re-exports. | "I'll add an index.ts for cleaner imports" |
| **`@/` imports only** | Use `@/components/...`, never `../../../`. | "Relative path is fine for now" |
| **One test per component** | Every new `.tsx` gets a `.test.tsx`. No exceptions. | "I'll add tests later" |
| **Responsive from birth** | Every component must not break at 375px width. | "I'll handle mobile later" |
| **Accessible by default** | Form inputs need labels. Images need alt text. Interactive elements need focus states. | "Accessibility can come later" |

**`.mcp.json`** — pre-wires Gasoline as an MCP server so tools are available immediately.

### 7. Dev Server Detection

After `pnpm dev` starts:
- Parse stdout for Vite's "Local: http://localhost:XXXX" message (regex: `Local:\s+http://localhost:(\d+)`)
- Fallback: poll ports 5173-5180 for HTTP 200
- Timeout after 30 seconds with error

### 8. Goal-Backward Verification

After Phase 2 completes, run a final verification pass. Don't check "did commands succeed" — check "can the user achieve the goal?"

**The goal:** User sees a meaningful first screen and can click an element to edit it.

**Verification chain (work backward from goal):**

1. **Functional:** Edit a component (change a Tailwind class), verify HMR updates the browser within 2 seconds
2. **Wired:** Components import each other correctly, Tailwind classes resolve, shadcn components render
3. **Substantive:** The UI reflects the user's description — not Vite boilerplate, not empty shells. Real layout, real content.
4. **Exists:** All expected files present (Layout, Header, feature components, CLAUDE.md, bootstrap skill)

If any level fails, the AI agent fixes it (same compose loop) before declaring completion.

### 9. WebSocket Architecture

The wizard connects via WebSocket on **port 7891** (existing terminal server), multiplexed with channel tags:

**Channels:**
- `scaffold` — Phase 1 step progress (step status, verification results)
- `compose` — Phase 2 AI composition progress (file writes, HMR, screenshots, fixes)
- `terminal` — raw PTY output (expandable for power users)
- `wizard` — user interactions (wizard answers, create action)

**Future consolidation:** port 7891 can be merged onto 7890 by exempting WebSocket connections from the main server's timeouts. Not needed for v1.

## Hooks (Generated Into Project)

The scaffold generates Claude Code hooks into the project's `.claude/` directory. These provide runtime intelligence during development sessions.

### Hook 1: Session Start — Bootstrap Context Injection

**Event:** `SessionStart` (fires on startup, `/clear`, context compaction)
**Pattern:** Superpowers' single-hook bootstrap

Injects the bootstrap skill as `additionalContext` so the AI agent always knows project conventions, even after context resets.

### Hook 2: Context Monitor — Warn Before Quality Degrades

**Event:** `PostToolUse` (fires after every tool call)
**Pattern:** GSD's two-stage bridge architecture

The status line hook writes context metrics to a bridge file. After every tool use, the context monitor reads it:
- **35% remaining** → warn: "Wrap up, commit, consider fresh session."
- **25% remaining** → critical: "Stop, save your work."
- Debounce: 5 tool uses between warnings (unless severity escalates)

### Hook 3: Status Line — Live Project Dashboard

**Event:** `statusLine` (continuous)

```
tracking: localhost:5173 | 2 errors | HMR: ok | context: [████████░░] 65%
```

Writes the context metrics bridge file that Hook 2 reads.

### What We Don't Generate

- **No PreToolUse guard** — don't nag users in a fresh project.
- **No update checker** — scaffold is one-time, not a versioned dependency.

## Prerequisites

- `node` >= 18 on PATH
- `pnpm` on PATH (wizard offers to install if missing: `npm install -g pnpm`)
- Gasoline daemon running (serves the wizard — always true if user opened `/launch`)
- Chrome with Gasoline extension installed (for annotation mode; Phase 1 works without it)

The wizard checks these before showing "Create" and shows clear, actionable errors.

## Quality Gates & Invariants

The scaffold ships with automated quality enforcement so the codebase stays clean from day one. No setup required — it's all in place before the user writes their first line.

### Pre-commit Hook (catches slop before it lands)

Generated as `.husky/pre-commit` (or simple git hook):

```bash
pnpm tsc --noEmit          # Type errors block commit
pnpm eslint --max-warnings 0  # No warnings, no exceptions
pnpm prettier --check .     # Formatting is non-negotiable
```

**Component invariant check** — a lightweight script (`scripts/check-components.sh`) that verifies:
- Every `.tsx` in `src/components/` has a default export
- No inline `style={}` attributes (Tailwind-first enforcement)
- No CSS module imports (use Tailwind, not `.module.css`)
- No `any` types
- No hardcoded hex colors in className strings (use theme tokens)
- No `../` imports (use `@/` path aliases)
- No files over 200 LOC

This is ~30 lines of grep, not a framework. Runs in <1 second.

### Vitest with Coverage Floor (catches regressions)

Pre-configured in Phase 1. Phase 2's AI agent writes one test per composed component.

- `vitest` runs on `pnpm test`
- Coverage threshold: **60% lines** (low bar, but enforced — prevents "zero tests forever")
- Pre-push hook runs `pnpm test` — broken tests block push
- Bootstrap skill instructs the AI to write tests for every new component
- `vitest.config.ts` pre-wired with happy-dom, path aliases, coverage reporter

### Pre-Deploy Security Scan (catches vulnerabilities before they ship)

Before any deploy, a security scan runs automatically. This is the last gate — nothing ships with known vulnerabilities.

**Frontend scan** (runs locally, no account needed):
- `pnpm audit` — checks dependencies for known CVEs, blocks on high/critical
- ESLint security rules (`eslint-plugin-security`) — catches common patterns: `eval()`, `innerHTML`, unsanitized URLs, prototype pollution
- Gasoline's own `analyze(what='security_audit')` — scans the running app for XSS vectors, insecure headers, mixed content, open redirects

**Backend scan** (when applicable):
- `pnpm audit` covers server-side deps too
- `.env` leak check — verifies no secrets in committed files (grep for API keys, tokens, passwords in source)
- CSP header generation — `generate(what='csp')` creates a Content Security Policy from observed traffic

**Cloud security scanner** (recommended partner — revenue share opportunity):

The scaffold recommends connecting a cloud scanner for ongoing monitoring. Pre-configured integration with partner TBD:

| Scanner | Fit | Partner Program |
|---------|-----|----------------|
| **Codacy** | Code quality + security in one. Auto-reviews PRs. | Revenue share via referral program |
| **Snyk** | Best dependency vulnerability scanning. Free tier generous. | Partner program for developer tools |
| **SonarCloud** | Deep static analysis, security hotspots. Free for open source. | Technology partner program |
| **Socket.dev** | Supply chain security — detects malicious packages. | Startup program |

The wizard's "Ready" screen and the deploy output both suggest connecting a scanner:

```
Security scan passed. 0 vulnerabilities found.

Tip: Connect a cloud scanner for continuous monitoring:
  → pnpm security:setup    (guided setup for Codacy/Snyk/SonarCloud)
```

`pnpm security:setup` is an interactive script that walks the user through connecting their preferred scanner, creates the config file, and (if they have a GitHub repo) sets up the GitHub App integration.

**Revenue share strategy:** Same model as deploy platforms — every Strum project recommends a scanner, negotiate referral terms with all four. Codacy and Snyk are the strongest candidates (both have active partner programs for dev tools).

### Auto-Commit Hook (keeps work saved)

The scaffold configures automatic commits to prevent lost work. A PostToolUse hook (or file watcher) detects meaningful changes and prompts:

**How it works:**
- After every 5 file saves (or 5 minutes of edits), the hook checks `git diff --stat`
- If there are uncommitted changes, it auto-commits with a descriptive message: `auto: update TodoList component and add drag handlers`
- Commits go to a local branch — never auto-pushed
- The user always sees: `[auto-saved 2 minutes ago]` in the status line

This ensures the user never loses more than a few minutes of work, even if they never think about git.

### GitHub Onboarding

The scaffold guides users toward GitHub for backup and collaboration.

**During Phase 1** (if `git` is on PATH):
- `git init` runs automatically (already in the scaffold)
- If `gh` CLI is installed, check if authenticated: `gh auth status`

**In the "Ready" screen** (after Phase 2):

```
Your app is running! Next steps:
  → Edit with annotation mode (click any element)
  → Deploy: pnpm deploy
  → Back up to GitHub: pnpm github:setup    (free, keeps your code safe)
```

**`pnpm github:setup`** is an interactive script:
1. Check if `gh` CLI is installed → if not, offer to install: `brew install gh` / link to download
2. Check if authenticated → if not, run `gh auth login`
3. Create a private repo: `gh repo create <project-name> --private --source=. --push`
4. Set up branch protection on `main` (require PR, require status checks)
5. If a deploy platform is connected, enable preview deploys on PR

**Revenue share:** GitHub doesn't have a direct referral program, but driving users to GitHub → GitHub Actions → GitHub Copilot is a natural funnel. Worth a conversation about co-marketing.

### The Invariant Chain

```
Edit code → auto-commit (every 5 saves or 5 minutes)
         → pre-commit (types + lint + format + component invariants)
         → push (tests + 60% coverage floor)
         → pre-deploy (security scan: audit + ESLint security + .env leak check)
         → deploy (one command)
         → cloud scanner (continuous monitoring via Codacy/Snyk/SonarCloud)
```

Every step is automated, zero-config, and blocks on failure. The user never sets up CI — it's in the git hooks from minute one.

## One-Command Deploy

The scaffold includes deploy configs for three platforms. Default TBD based on partnership terms (revenue share negotiations in progress with all three).

### Supported Platforms

**Vercel** (default candidate):
- `vercel.json` pre-configured for Vite+React (zero-config)
- `pnpm deploy:vercel` → `vercel --prod`
- Preview deploys on every push via GitHub integration
- Partner program offers revenue share for platforms driving signups

**Cloudflare Pages:**
- `wrangler.toml` pre-configured
- `pnpm deploy:cloudflare` → `wrangler pages deploy dist`
- Free tier is generous (unlimited bandwidth)
- Workers Launchpad program for startups

**Netlify:**
- `netlify.toml` pre-configured
- `pnpm deploy:netlify` → `netlify deploy --prod`
- Technology partner program with revenue share
- Hungry for market share, may offer best terms

### Deploy Setup

All three configs are generated. The wizard's "Ready" screen offers:

```
Your app is running! Next steps:
  → Edit with annotation mode (click any element)
  → Deploy: pnpm deploy        (first time sets up hosting)
```

`pnpm deploy` is an alias that auto-detects which platform is configured. First run is interactive (links project, creates account if needed). After that, it's one command.

### Revenue Share Strategy

Every Strum-scaffolded project recommends a deploy platform, security scanner, and GitHub. At scale, this is significant referral volume across three revenue streams.

#### Partner Revenue Breakdown

| Provider | Rev Share? | Model | Priority |
|----------|-----------|-------|----------|
| **Vercel** | Yes | % of referred paid plans via partner program | High — best React/Vite fit, active program |
| **Netlify** | Yes | Referral fees via technology partner program | High — hungry for share, may offer best terms |
| **Cloudflare** | Indirect | Workers Launchpad credits, co-marketing | Medium — generous free tier limits conversion |
| **Codacy** | Yes | % of first year via referral program | High — code quality + security in one, natural fit |
| **Snyk** | Yes | Referral fees on Team/Enterprise plans | High — best dependency scanning, strong partner program |
| **SonarCloud** | Likely | Partner program exists, terms negotiable | Medium — strong product, less established referral |
| **Socket.dev** | Unknown | Startup, likely negotiable | Low — niche (supply chain), small install base |
| **GitHub** | No | Co-marketing only, no per-user payments | Include anyway — essential UX, indirect funnel to Actions/Copilot |
| **GitLab** | Yes | Partner program with referral commissions | Backup — if GitHub co-marketing doesn't materialize |

#### Revenue Streams Per Scaffolded Project

```
Deploy platform:    $X/month per converted user (Vercel/Netlify)
Security scanner:   $Y/year per converted user (Codacy/Snyk)
Source hosting:     $0 (GitHub) — but co-marketing value + user stickiness
```

At 1,000 scaffolded projects/month, even 5% conversion to paid tiers generates meaningful recurring revenue across all three streams.

#### Negotiation Approach

1. Reach out to Vercel, Netlify, Codacy, Snyk simultaneously — leverage competition
2. Default to whichever gives best terms in each category (deploy, scanner)
3. Make all options available — user picks, but "Recommended" badge drives defaults
4. Track referral attribution via UTM params or partner-specific install flags
5. GitHub: pursue co-marketing partnership even without rev share — it's table stakes for the UX

## Production Observability (Post-Scaffold)

Gasoline captures everything during development — but what about after deploy? The scaffold pre-wires production observability so the user's app is never a black box.

### Error Tracking

The scaffold pre-installs a lightweight error boundary and recommends connecting an error tracking service:

**Built-in (no account needed):**
- React error boundary in `src/components/ErrorBoundary.tsx` — catches render crashes, shows friendly fallback
- `window.onerror` + `window.onunhandledrejection` handlers in `src/lib/error-reporter.ts`
- During dev: errors flow to Gasoline automatically (already works)
- During production: errors log to console (baseline) or forward to a service (if connected)

**Recommended service (wizard Step 7b — alongside security scanner):**

| Service | Fit | Rev Share |
|---------|-----|-----------|
| **Sentry** | Industry standard error tracking. Generous free tier (5K events/month). | Yes — partner program, referral fees |
| **BetterStack** | Error tracking + uptime + logging in one. Clean UI. | Yes — partner program |
| **LogRocket** | Session replay + error tracking. See what user did before crash. | Yes — referral program |

The wizard offers this as part of the infrastructure setup:

*"We'll catch errors automatically during development. Want to track errors in production too? (Free to start)"*

One-click connect, pre-wired in the scaffold. The error reporter checks for a `VITE_SENTRY_DSN` (or equivalent) env var — if set, errors forward to the service; if not, console-only.

### Analytics

For "My team" and "Public users" audiences (wizard Step 2), the scaffold offers basic analytics:

**Recommended service:**

| Service | Fit | Rev Share |
|---------|-----|-----------|
| **PostHog** | Product analytics + session replay. Open source option. Generous free tier. | Yes — partner program |
| **Plausible** | Privacy-first, lightweight. Good for simple sites. | Affiliate program |
| **Umami** | Self-hosted option. Free forever. | No (open source) |

Pre-wired as `src/lib/analytics.ts` — a thin wrapper that no-ops if no service is configured. Never blocks rendering. GDPR-friendly defaults (no cookies until consent).

### Uptime Monitoring

For deployed apps, the wizard recommends uptime monitoring:

*"Want to know if your app goes down? We'll ping it every minute. (Free to start)"*

| Service | Fit | Rev Share |
|---------|-----|-----------|
| **BetterStack** | Uptime + incident pages + alerting. Beautiful status pages. | Yes — partner program |
| **UptimeRobot** | Simple, free for 50 monitors. | Affiliate program |

One-click setup creates a monitor for the deploy URL.

### Revenue Opportunity

Every scaffolded project that connects to production services = recurring referral revenue across up to 6 partners:

```
Deploy:      Vercel / Netlify / Cloudflare     (1 per project)
Security:    Codacy / Snyk / SonarCloud        (1 per project)
Errors:      Sentry / BetterStack / LogRocket  (1 per project)
Analytics:   PostHog / Plausible               (1 per project, team/public only)
Uptime:      BetterStack / UptimeRobot         (1 per project)
Source:      GitHub                            (co-marketing)
```

At scale, a single scaffolded project can generate referral revenue from 4-5 services simultaneously.

## Learn While You Build

The wizard and AI composition phases are teaching moments. Users are watching their app get built — that's when they're most receptive to learning *why* things are done a certain way.

### Contextual Lessons During Phase 2

As the AI agent composes each component, the wizard UI shows a brief lesson panel alongside the progress:

```
┌─ Building your app ──────────────────────────────────────┐
│                                                          │
│  ✓ Created Layout.tsx                                    │
│  → Building Header.tsx                                   │
│                                                          │
│  ┌─ Learn ────────────────────────────────────────────┐  │
│  │ 💡 Components                                      │  │
│  │                                                    │  │
│  │ The AI just created a "Header" component — a       │  │
│  │ reusable piece of UI. In React, you build apps     │  │
│  │ by composing small components together, like       │  │
│  │ LEGO bricks. Each component is one file.           │  │
│  │                                                    │  │
│  │ Your Header component is at:                       │  │
│  │ src/components/Header.tsx                          │  │
│  │                                                    │  │
│  │ Try clicking it in the browser after we're done    │  │
│  │ — annotation mode will show you the code.          │  │
│  └────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

### Lesson Catalog

Each lesson is triggered by what the AI agent is doing at that moment:

| AI Action | Lesson Topic | Key Concept |
|-----------|-------------|-------------|
| Writing Layout.tsx | **Components** | Apps are built from small reusable pieces |
| Adding Tailwind classes | **Styling with Tailwind** | Utility classes instead of writing CSS files |
| Importing shadcn Button | **Component libraries** | Don't build everything from scratch |
| Adding `useState` | **State** | How components remember things |
| Writing a `.test.tsx` | **Testing** | How to know your code works |
| Checking mobile viewport | **Responsive design** | Making apps work on any screen size |
| Running accessibility check | **Accessibility** | Building apps everyone can use |
| Using `@/` imports | **Project structure** | How files are organized and why |
| Setting up error boundary | **Error handling** | What happens when things go wrong |
| Adding environment variables | **Configuration** | Keeping secrets out of code |

### Lesson Design Principles

- **3 sentences max.** If it takes longer to explain, link to a full guide.
- **Show the file path.** Always point to where the concept lives in their code.
- **End with an action.** "Try clicking it", "Change the color and watch it update", "Run `pnpm test` to see it pass."
- **Never block progress.** Lessons appear alongside composition, never pause it.
- **Dismissable.** "Got it" button collapses the lesson. "Don't show tips" disables all.
- **Progressive.** First project gets all lessons. If we detect the user has built before (cookie/localStorage), show fewer.

### Post-Composition Learning Path

After Phase 2 completes, the "Ready" screen includes a learning section:

```
Your app is running! Here's what was built:

📁 Project structure
  src/components/  — your UI components (click any to edit)
  src/lib/         — shared utilities
  src/App.tsx      — the main entry point

🎯 Try these first:
  1. Click any element in the browser → see the code that makes it
  2. Change a color class (e.g., bg-blue-500 → bg-red-500) → watch it update instantly
  3. Run "pnpm test" in terminal → see your tests pass
  4. Run "pnpm deploy" → put it on the internet

📚 Want to learn more?
  → React basics (link)
  → Tailwind CSS guide (link)
  → How components work (link)
```

### Annotation Mode as a Teaching Tool

Annotation mode is inherently educational — the user clicks a UI element and sees the source code that produces it. The scaffold's bootstrap skill enhances this:

- When the user clicks an element, the AI agent explains what the code does (if they ask)
- The agent references the relevant lesson: "This uses `useState` — remember the State lesson from when we built it?"
- Component names in the source match what was shown during composition, creating continuity

This turns every editing session into an implicit tutorial. The user learns React/Tailwind/TypeScript by doing, not by reading docs.

## Entry Point: Extension Popup

The scaffold wizard is discoverable from the extension popup's "no tab tracked" state — the cold-start moment when the user has Gasoline installed but nothing to point it at.

**When no tab is tracked, the popup shows two buttons:**

```
┌─────────────────────────────────┐
│  Tab Control                    │
│                                 │
│  ┌───────────────────────────┐  │
│  │     Track This Tab        │  │  ← primary, existing behavior
│  └───────────────────────────┘  │
│  Click to start monitoring      │
│  this tab's console, network,   │
│  and errors                     │
│                                 │
│  ┌───────────────────────────┐  │
│  │    Build Something New    │  │  ← secondary, navigates to /launch
│  └───────────────────────────┘  │
│  Create a new app from scratch  │
│  with AI                        │
└─────────────────────────────────┘
```

**When a tab is tracked:** both buttons disappear, replaced by the tracking bar and post-lovable tools (recording, capture controls, etc.). The `/launch` URL still works for power users who want to create another project.

**"Build Something New" behavior:** opens `localhost:7890/launch` in a new tab and closes the popup. The wizard takes over from there.

## Open Questions

- **Error recovery**: if Phase 1 fails mid-way, clean up partial directory or leave for debugging?
- **Project gallery**: should `/launch` also list existing strum-projects for re-opening?
- **Port conflicts**: if 5173 is taken, Vite auto-picks another — need to parse actual port from stdout
- **Template variants**: start with one template (React + shadcn), add more later?
- **AI agent selection**: which model for Phase 2 composition? Needs to be fast (Sonnet?) but capable of good UI decisions
- **Offline Phase 2**: can Phase 2 work without an API key? (fallback: static template instead of AI composition)
- **Pencil integration**: offer a "Design first in Pencil → export → scaffold with that design" flow? Pencil has a shadcn design kit
- **Fine-grained streaming**: use Anthropic's `fine-grained-tool-streaming` beta so file writes stream in real-time?
- **Composition depth**: how many components should Phase 2 create? Just the first feature, or a full page?
- **Deploy default**: which platform to default to? Depends on rev share negotiations
- **Security scanner default**: which scanner to recommend? Depends on rev share negotiations with Codacy/Snyk/SonarCloud/Socket
- **Auto-commit frequency**: every 5 saves / 5 minutes — is this too aggressive? Too quiet? Should it be configurable?
- **GitHub push**: should `pnpm github:setup` also set up GitHub Actions CI, or are git hooks enough for v1?
- **CI generation**: should we generate a GitHub Actions workflow that runs the full invariant chain on every PR?

## Success Criteria

- Phase 1 (scaffold to running dev server): < 20 seconds
- Phase 2 (AI composition to meaningful first screen): < 60 seconds
- Total wizard to editable app: < 90 seconds on broadband
- The first screen reflects the user's description — not boilerplate
- Each Phase 2 component is visible in the browser within 3 seconds of being written (HMR)
- Screenshot verification catches and fixes layout issues before declaring completion
- Annotation mode works immediately on all composed components
- All generated code passes `tsc --noEmit` and `eslint` with zero errors
- Pre-commit hook blocks commits with type errors, lint warnings, formatting issues, or component invariant violations
- Component invariant check blocks inline styles, `any` types, hardcoded hex colors, `../` imports, and files over 200 LOC
- Vitest runs with 60% coverage floor enforced on push
- Phase 2 composition passes accessibility audit (`analyze(what='accessibility')`) with zero WCAG violations
- Phase 2 composition passes mobile responsive check (375px viewport, no broken layouts)
- All composed components use shadcn where applicable, Tailwind classes only, theme tokens only, Lucide icons only
- Pre-deploy security scan blocks on high/critical CVEs and catches XSS, secrets in source, missing CSP
- `pnpm deploy` works first try with no manual config on all three platforms
- `pnpm github:setup` creates a private repo and pushes in one interactive flow
- Auto-commit keeps work saved — user never loses more than 5 minutes of edits
- `pnpm security:setup` walks user through connecting a cloud scanner
- User never needs to touch a terminal — everything happens from the wizard
- Goal-backward verification passes: Exists → Substantive → Wired → Functional
- Session-start hook survives `/clear` — agent always knows project conventions
- Context monitor fires warnings at 35% and 25% remaining
