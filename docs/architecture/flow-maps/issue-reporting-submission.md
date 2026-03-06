---
doc_type: flow_map
flow_id: issue-reporting-submission
status: active
last_reviewed: 2026-03-05
owners:
  - Brenn
feature_ids:
  - feature-issue-reporting
entrypoints:
  - cmd/dev-console/tools_configure_report_issue.go:toolConfigureReportIssue
code_paths:
  - cmd/dev-console/tools_configure_report_issue.go
  - cmd/dev-console/tools_configure_registry.go
  - internal/issuereport/types.go
  - internal/issuereport/templates.go
  - internal/issuereport/sanitize.go
  - internal/issuereport/submit.go
test_paths:
  - cmd/dev-console/tools_configure_report_issue_test.go
  - internal/issuereport/templates_test.go
  - internal/issuereport/sanitize_test.go
  - internal/issuereport/submit_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Issue Reporting Submission Flow

## Scope

Covers the end-to-end flow from `configure(what="report_issue")` through diagnostics collection, sanitization, and submission via `gh` CLI.

## Entrypoints

- `toolConfigureReportIssue()` in `cmd/dev-console/tools_configure_report_issue.go`

## Primary Flow

1. `toolConfigure()` dispatches to `configureHandlers["report_issue"]` in registry.
2. `toolConfigureReportIssue()` parses params: `operation`, `template`, `title`, `user_context`.
3. Routes to one of three sub-flows based on `operation`:
   - `list_templates` → returns 5 hardcoded templates via `issuereport.TemplateNames()` and `issuereport.GetTemplate()`.
   - `preview` (default) → step 4.
   - `submit` → step 4, then step 7.
4. `collectIssueReport()` gathers diagnostics:
   - Server version, platform from Go runtime.
   - Uptime and audit stats from `healthMetrics`.
   - Extension connectivity from `capture.GetHealthSnapshot()`.
   - Buffer counts from capture and server.
5. `sanitizeIssueReport()` creates `issuereport.Sanitizer` backed by `RedactionEngine`.
6. `Sanitizer.SanitizeReport()` applies `Redact()` to title, user_context, extension source.
7. (submit only) `issuereport.SubmitViaGH()`:
   - Checks `exec.LookPath("gh")`.
   - If found: runs `gh issue create --repo ... --title ... --body ... --label ...` with 30s timeout.
   - Parses issue URL from stdout.

## Error and Recovery Paths

| Condition | Behavior |
|-----------|----------|
| Unknown operation | Returns `ErrInvalidParam` with valid operations list |
| Unknown template | Returns `ErrInvalidParam` with hint to use `list_templates` |
| Submit without title | Returns `ErrMissingParam` |
| `gh` not installed | Returns `{status: "manual", formatted_body, repo_url}` |
| `gh` fails (auth, network) | Returns `{status: "error", error, formatted_body}` for manual fallback |
| Invalid JSON args | Returns `ErrInvalidJSON` |
| Nil redaction engine | Skips sanitization (returns report as-is) |

## State and Contracts

- **Privacy invariant**: Default operation is `preview`. Submit requires explicit `operation="submit"` + `title`.
- **Redaction contract**: All user-supplied strings pass through `RedactionEngine.Redact()` before leaving the machine.
- **No background submission**: Synchronous only, no auto-reporting.
- **CommandRunner interface**: Enables test injection for `exec.LookPath` and `exec.CommandContext`.

## Code Paths

- `cmd/dev-console/tools_configure_report_issue.go`
- `cmd/dev-console/tools_configure_registry.go`
- `internal/issuereport/types.go`
- `internal/issuereport/templates.go`
- `internal/issuereport/sanitize.go`
- `internal/issuereport/submit.go`

## Test Paths

- `cmd/dev-console/tools_configure_report_issue_test.go`
- `internal/issuereport/templates_test.go`
- `internal/issuereport/sanitize_test.go`
- `internal/issuereport/submit_test.go`

## Edit Guardrails

1. Adding a new template requires updating the `templates` map in `templates.go` and test assertions (`TemplateNames()` is derived from map keys automatically).
2. Adding new diagnostic fields requires updating `collectIssueReport()`, `DiagnosticData` type, and `FormatIssueBody()`.
3. Changing the `RedactionEngine` interface requires updating all mocks in `handler_unit_test.go`.
4. The `Redactor` interface in `sanitize.go` must stay in sync with `RedactionEngine` in `handler.go`.
