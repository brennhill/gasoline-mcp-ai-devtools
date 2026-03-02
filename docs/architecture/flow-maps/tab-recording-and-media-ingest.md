---
doc_type: flow_map
flow_id: tab-recording-and-media-ingest
status: active
last_reviewed: 2026-03-02
owners:
  - Brenn
entrypoints:
  - cmd/dev-console/tools_interact_dispatch.go (record_start|record_stop)
  - cmd/dev-console/server_routes.go (/screenshots|/draw-mode/complete)
  - cmd/dev-console/tools_observe.go (saved_videos)
code_paths:
  - cmd/dev-console/tools_recording_video.go
  - cmd/dev-console/tools_recording_video_paths.go
  - cmd/dev-console/tools_recording_video_state.go
  - cmd/dev-console/tools_recording_video_handlers.go
  - cmd/dev-console/tools_recording_video_save.go
  - cmd/dev-console/tools_recording_video_reveal.go
  - cmd/dev-console/tools_recording_video_observe.go
  - cmd/dev-console/server_routes_media_common.go
  - cmd/dev-console/server_routes_media_screenshots.go
  - cmd/dev-console/server_routes_media_draw_mode.go
test_paths:
  - cmd/dev-console/tools_recording_video_test.go
  - cmd/dev-console/server_routes_unit_test.go
  - cmd/dev-console/tools_draw_mode_http_test.go
  - cmd/dev-console/annotation_store_test.go
---

# Tab Recording and Media Ingest

## Scope

Covers interact record_start/record_stop state machine, screenshot ingest, draw-mode ingest, and saved video listing/reveal behavior.

## Entrypoints

- Interact actions: `record_start`, `record_stop`.
- HTTP media endpoints: `/screenshots`, `/draw-mode/complete`, `/recordings/save`, `/recordings/reveal`, `/recordings/storage`.
- Observe query: `saved_videos`.

## Primary Flow

1. `handleRecordStart` validates extension readiness, clamps FPS/audio, resolves path, and queues extension command.
2. Recording state transitions are derived from command results in `resolveInteractRecordingState`.
3. `handleRecordStop` enforces valid state before queueing stop command.
4. `/screenshots` validates rate limits and data URLs, then persists image and optional query result payload.
5. `/draw-mode/complete` stores screenshot, annotations, details, and pushes completion updates.
6. `toolObserveSavedVideos` enumerates persisted video metadata across primary + legacy dirs.

## Error and Recovery Paths

- Invalid audio mode, malformed data URL, invalid path, or missing tab ID return structured errors.
- Screenshot limiter returns `429` on per-client burst, `503` on capacity exhaustion.
- Draw-mode parse failures are returned as warnings while valid annotations still persist.

## State and Contracts

- Recording lifecycle constants: `idle`, `awaiting_user_gesture`, `recording`, `stopping`.
- `recordInteract` state is guarded by `recordInteractMu`.
- File path writes must remain under runtime state dirs (`pathWithinDir` / `isWithinDir`).

## Code Paths

- `cmd/dev-console/tools_recording_video.go`
- `cmd/dev-console/tools_recording_video_paths.go`
- `cmd/dev-console/tools_recording_video_state.go`
- `cmd/dev-console/tools_recording_video_handlers.go`
- `cmd/dev-console/tools_recording_video_save.go`
- `cmd/dev-console/tools_recording_video_reveal.go`
- `cmd/dev-console/tools_recording_video_observe.go`
- `cmd/dev-console/server_routes_media_common.go`
- `cmd/dev-console/server_routes_media_screenshots.go`
- `cmd/dev-console/server_routes_media_draw_mode.go`

## Test Paths

- `cmd/dev-console/tools_recording_video_test.go`
- `cmd/dev-console/server_routes_unit_test.go`
- `cmd/dev-console/tools_draw_mode_http_test.go`
- `cmd/dev-console/annotation_store_test.go`

## Edit Guardrails

- Keep state-machine transitions deterministic and test-backed.
- Any media endpoint payload change must update endpoint contract tests.
- Preserve directory-bound path validation before writes or reveals.
