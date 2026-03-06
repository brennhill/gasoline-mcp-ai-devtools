// Purpose: Checkpoint-based telemetry diffing and alert lifecycle.
// Why: Isolates incremental state comparison from unrelated noise and persistence concerns.
// Docs: docs/features/feature/push-alerts/index.md

/*
Package checkpoint computes incremental diffs across console/network/websocket/action
streams and tracks alert delivery state for checkpoint windows.
*/
package checkpoint
