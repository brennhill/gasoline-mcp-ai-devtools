---
doc_type: reference
status: active
scope: core/mcp-contract/matrix
ai-priority: high
tags: [core, mcp, command-matrix, option-traceability, canonical]
relates-to: [product-spec.md, tech-spec.md, qa-spec.md]
last-verified: 2026-02-17
canonical: true
---

# MCP Command and Option Matrix (TARGET)

## Source of Truth
- Schemas: `cmd/dev-console/tools_schema.go`
- Dispatchers: `cmd/dev-console/tools_*.go`
- Extension execution: `src/background/pending-queries.ts`

## Command Traceability

### `observe` (`what` -> handler)
- `errors` -> `toolGetBrowserErrors`
- `logs` -> `toolGetBrowserLogs`
- `extension_logs` -> `toolGetExtensionLogs`
- `network_waterfall` -> `toolGetNetworkWaterfall`
- `network_bodies` -> `toolGetNetworkBodies`
- `websocket_events` -> `toolGetWSEvents`
- `websocket_status` -> `toolGetWSStatus`
- `actions` -> `toolGetEnhancedActions`
- `vitals` -> `toolGetWebVitals`
- `page` -> `toolGetPageInfo`
- `tabs` -> `toolGetTabs`
- `pilot` -> `toolObservePilot`
- `timeline` -> `toolGetSessionTimeline`
- `error_bundles` -> `toolGetErrorBundles`
- `screenshot` -> `toolGetScreenshot`
- `command_result` -> `toolObserveCommandResult`
- `pending_commands` -> `toolObservePendingCommands`
- `failed_commands` -> `toolObserveFailedCommands`
- `saved_videos` -> `toolObserveSavedVideos`
- `recordings` -> `toolGetRecordings`
- `recording_actions` -> `toolGetRecordingActions`
- `playback_results` -> `toolGetPlaybackResults`
- `log_diff_report` -> `toolGetLogDiffReport`

### `analyze` (`what` -> handler -> query type)
- `dom` -> `toolQueryDOM` -> `dom`
- `performance` -> `toolCheckPerformance` -> server-side
- `accessibility` -> `toolRunA11yAudit` -> `a11y`
- `error_clusters` -> `toolAnalyzeErrors` -> server-side
- `history` -> `toolAnalyzeHistory` -> server-side
- `security_audit` -> `toolSecurityAudit` -> server-side
- `third_party_audit` -> `toolAuditThirdParties` -> server-side
- `link_health` -> `toolAnalyzeLinkHealth` -> `link_health`
- `link_validation` -> `toolValidateLinks` -> server-side HTTP
- `page_summary` -> `toolAnalyzePageSummary` -> `execute`
- `annotations` -> `toolGetAnnotations` -> annotation store/waiter path
- `annotation_detail` -> `toolGetAnnotationDetail` -> annotation store
- `api_validation` -> `toolValidateAPI` -> server-side analyzer
- `draw_history` -> `toolListDrawHistory` -> persisted files
- `draw_session` -> `toolGetDrawSession` -> persisted files

### `configure` (`action` -> handler)
- `store` -> `toolConfigureStore`
- `load` -> `toolLoadSessionContext`
- `noise_rule` -> `toolConfigureNoiseRule`
- `clear` -> `toolConfigureClear`
- `health` -> `toolGetHealth`
- `streaming` -> `toolConfigureStreamingWrapper`
- `test_boundary_start` -> `toolConfigureTestBoundaryStart`
- `test_boundary_end` -> `toolConfigureTestBoundaryEnd`
- `recording_start` -> `toolConfigureRecordingStart`
- `recording_stop` -> `toolConfigureRecordingStop`
- `playback` -> `toolConfigurePlayback`
- `log_diff` -> `toolConfigureLogDiff`
- `telemetry` -> `toolConfigureTelemetry`
- `diff_sessions` -> `toolDiffSessionsWrapper`
- `audit_log` -> `toolGetAuditLog`

### `interact` (`action` -> handler -> query type)
- `highlight` -> `handlePilotHighlight` -> `highlight`
- `subtitle` -> `handleSubtitle` -> `subtitle`
- `save_state` -> `handlePilotManageStateSave` -> state path
- `load_state` -> `handlePilotManageStateLoad` -> state path
- `list_states` -> `handlePilotManageStateList` -> state path
- `delete_state` -> `handlePilotManageStateDelete` -> state path
- `execute_js` -> `handlePilotExecuteJS` -> `execute`
- `navigate` -> `handleBrowserActionNavigate` -> `browser_action`
- `refresh` -> `handleBrowserActionRefresh` -> `browser_action`
- `back` -> `handleBrowserActionBack` -> `browser_action`
- `forward` -> `handleBrowserActionForward` -> `browser_action`
- `new_tab` -> `handleBrowserActionNewTab` -> `browser_action`
- `screenshot` -> `handleScreenshotAlias` -> observe screenshot path
- `click|type|select|check|get_text|get_value|get_attribute|set_attribute|focus|scroll_to|wait_for|key_press|paste` -> `handleDOMPrimitive` -> `dom_action`
- `list_interactive` -> `handleListInteractive` -> `dom_action`
- `record_start` -> `handleRecordStart` -> `record_start`
- `record_stop` -> `handleRecordStop` -> `record_stop`
- `upload` -> `handleUpload` -> `upload`
- `draw_mode_start` -> `handleDrawModeStart` -> `draw_mode`

### `generate` (`format` -> handler)
- `reproduction` -> `toolGetReproductionScript`
- `test` -> `toolGenerateTest`
- `pr_summary` -> `toolGeneratePRSummary`
- `har` -> `toolExportHAR`
- `csp` -> `toolGenerateCSP`
- `sri` -> `toolGenerateSRI`
- `sarif` -> `toolExportSARIF`
- `visual_test` -> `toolGenerateVisualTest`
- `annotation_report` -> `toolGenerateAnnotationReport`
- `annotation_issues` -> `toolGenerateAnnotationIssues`
- `test_from_context` -> `handleGenerateTestFromContext`
- `test_heal` -> `handleGenerateTestHeal`
- `test_classify` -> `handleGenerateTestClassify`

## Option Traceability

### `observe` options
- Dispatch key: `what`
- Filtering/pagination keys: `limit`, `after_cursor`, `before_cursor`, `since_cursor`, `restart_on_eviction`, `min_level`, `level`, `source`, `url`, `method`, `status_min`, `status_max`, `body_key`, `body_path`, `connection_id`, `direction`, `last_n`, `include`, `window_seconds`, `recording_id`, `correlation_id`, `original_id`, `replay_id`
- Cross-cutting key: `telemetry_mode`

### `analyze` options
- Dispatch key: `what`
- Async control keys: `sync`, `wait`, `background`, `correlation_id`
- Mode-specific keys: `selector`, `frame`, `operation`, `ignore_endpoints`, `scope`, `tags`, `force_refresh`, `domain`, `timeout_ms`, `world`, `tab_id`, `max_workers`, `checks`, `severity_min`, `first_party_origins`, `include_static`, `custom_lists`, `session`, `urls`, `file`
- Cross-cutting key: `telemetry_mode`

### `generate` options
- Dispatch key: `format`
- Shared generation keys: `error_message`, `last_n`, `base_url`, `include_screenshots`, `generate_fixtures`, `visual_assertions`, `test_name`, `assert_network`, `assert_no_errors`, `assert_response_shape`, `scope`, `include_passes`, `save_to`, `url`, `method`, `status_min`, `status_max`, `mode`, `include_report_uri`, `exclude_origins`, `resource_types`, `origins`, `session`
- Test-heal/classify keys: `context`, `action`, `test_file`, `test_dir`, `broken_selectors`, `auto_apply`, `failure`, `failures`, `error_id`, `include_mocks`, `output_format`
- Cross-cutting key: `telemetry_mode`

### `configure` options
- Dispatch key: `action`
- Action family keys: `store_action`, `namespace`, `key`, `data`, `noise_action`, `rules`, `rule_id`, `pattern`, `category`, `reason`, `buffer`, `tab_id`, `session_action`, `name`, `compare_a`, `compare_b`, `recording_id`, `session_id`, `tool_name`, `since`, `limit`, `streaming_action`, `events`, `throttle_seconds`, `severity_min`, `test_id`, `label`, `original_id`, `replay_id`
- Cross-cutting key: `telemetry_mode`

### `interact` options
- Dispatch key: `action`
- Async control keys: `sync`, `wait`, `background`, `correlation_id`
- Action keys: `selector`, `frame`, `duration_ms`, `snapshot_name`, `include_url`, `script`, `timeout_ms`, `text`, `subtitle`, `value`, `clear`, `checked`, `name`, `audio`, `fps`, `world`, `url`, `tab_id`, `reason`, `analyze`, `session`, `file_path`, `api_endpoint`, `submit`, `escalation_timeout_ms`
- Cross-cutting key: `telemetry_mode`

## End-to-End Processing Layers
1. Schema exposure (`tools/list`) from `tools_schema.go`.
2. Tool dispatch and option parsing in `tools_*.go` handlers.
3. Extension-bound options forwarded inside queued query `Params` JSON.
4. Extension execution and result correlation in `pending-queries.ts` + `/sync`.
5. Final command status surfaced via `observe(command_result)`.
