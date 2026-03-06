// Purpose: Noise rule management, filtering, and proposal generation.
// Why: Keeps suppression logic modular and testable independent of persistence/checkpointing.
// Docs: docs/features/feature/noise-filtering/index.md

/*
Package noise maintains built-in and user-defined filtering rules for console, network,
and websocket telemetry, including confidence-based auto-detection.
*/
package noise
