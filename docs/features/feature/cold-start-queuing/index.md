# Cold-Start Queuing

## What it does

The readiness gate holds incoming tool commands for up to `coldStartTimeout`
(default **5 seconds**) while waiting for the browser extension's first `/sync`
heartbeat. This eliminates `no_data` failures when an LLM sends a command
before the extension has finished booting.

The gate is implemented in `requireExtension()`, which every interact/analyze
handler calls before dispatching a command. It delegates to
`capture.WaitForExtensionConnected(ctx, timeout)` which polls at 100ms
intervals using a `time.Ticker` and respects both the timeout and a
`context.Context` for cancellation (preventing goroutine leaks on shutdown).

## Default timeout

- **5 seconds** (`defaultColdStartTimeout` in `cmd/dev-console/tools_core.go`)

## How to disable

Set `coldStartTimeout = 0` to restore instant-fail behavior. This is the
default in test environments (`newGateTestEnv` sets it to 0).

In production, the timeout can be configured via the `ToolHandler.coldStartTimeout`
field.

## Interaction with background/async mode

When `background: true` is passed in the tool arguments, `MaybeWaitForCommand`
returns a `"queued"` response immediately **without** checking extension
connectivity. The cold-start gate is bypassed entirely for background commands.

## Interaction with fast-fail gate (#261)

The `requireExtension` gate runs **after** `requirePilot` but **before**
other gates (`requireTabTracking`, `requireCSPClear`). If the extension
connects within the timeout, subsequent gates evaluate normally. If it
times out, the structured error includes `retryable: true` and
`retry_after_ms: 3000` so the LLM can retry after the extension finishes
booting.

## Architecture

1. **Single gate** (`requireExtension`) -- the cold-start wait happens once
   per handler invocation. `MaybeWaitForCommand` performs only an instant
   `IsExtensionConnected()` check to catch disconnections between the gate
   and command dispatch (P1-2 fix: no double wait).

2. **Context-aware** -- `WaitForExtensionConnected` accepts a
   `context.Context` so the wait is cancelled if the MCP transport closes
   (P1-1 fix: prevents goroutine leaks on shutdown).

3. **Polling-based** -- The current implementation polls at 100ms intervals.
   A future improvement (tracked as TODO in the code) will replace polling
   with `sync.Cond` broadcast from `HandleSync` for zero-latency notification.
