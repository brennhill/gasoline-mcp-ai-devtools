// Purpose: Package recording — user flow recording, disk persistence, playback engine, and log diffing.
// Why: Preserves replayable execution history and enables before/after comparison of recorded sessions.
// Docs: docs/features/feature/playback-engine/index.md

/*
Package recording implements browser session recording, storage, and replay functionality.

Key types:
  - RecordingManager: manages recording lifecycle (start/stop), disk persistence, and storage quotas.
  - Recording: a captured user flow with actions, viewport info, and metadata.
  - RecordingAction: a single user action (click, type, navigate) with selector and coordinates.
  - PlaybackResult: result of executing a single action during replay.
  - LogDiffResult: comparison output between original and replayed recordings.

Key functions:
  - NewRecordingManager: creates a manager with configurable storage paths.
  - StartRecording: begins capturing user actions from the extension.
  - StopRecording: finalizes and persists a recording to disk.
  - CompareRecordings: diffs two recordings to detect regressions or fixes.
*/
package recording
