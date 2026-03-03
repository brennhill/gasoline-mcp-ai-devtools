---
doc_type: feature_index
feature_id: feature-file-upload
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-03-03
code_paths:
  - cmd/dev-console/tools_interact_upload.go
  - cmd/dev-console/upload_handlers.go
  - internal/upload/handlers.go
  - internal/upload/security.go
  - internal/upload/os_automation.go
test_paths:
  - cmd/dev-console/tools_interact_upload_test.go
  - cmd/dev-console/upload_handlers_test.go
  - internal/upload/security_test.go
  - internal/upload/os_automation_test.go
---

# File Upload

## TL;DR
- Status: shipped
- Tool: `interact`
- Action: `upload`

## Specs
- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Canonical Note
Upload is security-first: path validation and policy checks must pass before any OS-level dialog automation runs.
