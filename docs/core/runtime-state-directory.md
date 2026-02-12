# Runtime State Directory

## Problem

Gasoline previously wrote several runtime files directly into the user home root (for example `~/gasoline-logs.jsonl` and `~/.gasoline-7890.pid`).

This caused two issues:

1. Home-directory clutter and user complaints.
2. Unclear ownership of files when Gasoline runs as a long-lived server process independent of any single project checkout.

## Decision

Gasoline runtime artifacts now live under a dedicated **runtime state directory**:

- Default: OS app-state/config location + `gasoline`
  - Linux: `$XDG_STATE_HOME/gasoline` when `XDG_STATE_HOME` is set, otherwise `~/.config/gasoline`
  - macOS: `~/Library/Application Support/gasoline`
  - Windows: `%AppData%\gasoline`
- Override:
  - Env var: `GASOLINE_STATE_DIR`
  - CLI flag: `--state-dir`

## Layout

Within the runtime state directory:

- `logs/gasoline.jsonl` — server lifecycle and debug logs
- `logs/crash.log` — panic crash output
- `run/gasoline-<port>.pid` — per-port PID file
- `recordings/` — saved recording metadata and video sidecars
- `settings/extension-settings.json` — extension settings cache
- `security/security.json` — security policy configuration path

## Repo Root Handling

Gasoline should not assume it can discover a repository root:

- It runs as a shared server process, often separate from project processes.
- Clients may come and go across multiple workspaces.

For this reason, runtime server state is **server-scoped**, not repo-scoped, and is anchored to the runtime state directory above. Project-local artifacts that are intentionally repo-scoped (for example `.gasoline/` inside a project) remain project-owned and separate from server runtime state.

## Compatibility

Gasoline keeps compatibility reads/fallbacks for historical paths where practical:

- Legacy PID paths are still checked for stop/cleanup.
- Legacy settings and recordings can still be read.
- New writes go to the runtime state directory.
