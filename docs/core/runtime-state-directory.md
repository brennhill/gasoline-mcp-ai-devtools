---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Runtime State Directory

## Problem

Gasoline previously wrote several runtime files directly into the user home root (for example `~/gasoline-logs.jsonl` and `~/.gasoline-7890.pid`).

This caused two issues:

1. Home-directory clutter and user complaints.
2. Unclear ownership of files when Gasoline runs as a long-lived server process independent of any single project checkout.

## Decision

All Gasoline runtime artifacts now live under a single dotfolder: **`~/.gasoline`**.

Resolution order:

1. `GASOLINE_STATE_DIR` env var (if set) — overrides everything
2. `XDG_STATE_HOME/gasoline` (if `XDG_STATE_HOME` is set — Linux users who set this expect it)
3. `~/.gasoline` — cross-platform default

This works the same on macOS, Linux, and Windows, and is more discoverable than platform-specific paths like `~/Library/Application Support/gasoline`.

## Layout

```
~/.gasoline/
  logs/
    gasoline.jsonl          # server lifecycle and debug logs
    crash.log               # panic crash output
  run/
    gasoline-<port>.pid     # per-port PID file
  recordings/               # saved recording metadata and video sidecars
  screenshots/              # captured screenshots
  settings/
    extension-settings.json # extension settings cache
  security/
    security.json           # security policy configuration
  projects/
    Users/brenn/dev/myapp/  # mirrored abs path (minus leading /)
      meta.json
      baselines/
      noise/
      errors/
      ...
```

## Project-Scoped Persistence

AI persistence data is stored centrally under `~/.gasoline/projects/{abs-path}/` (with the leading `/` stripped from the absolute project path). This keeps all Gasoline state in one place rather than scattering `.gasoline/` directories inside each project.

## Compatibility

Legacy paths from earlier versions are still checked as read-only fallbacks:

- `~/Library/Application Support/gasoline` (macOS), `%AppData%\gasoline` (Windows), `~/.config/gasoline` (Linux) — the previous primary location
- `~/gasoline-logs.jsonl`, `~/gasoline-crash.log`, `~/.gasoline-7890.pid`, `~/.gasoline-settings.json` — pre-dotfolder flat files

New writes always go to `~/.gasoline`. No automatic migration is performed.
