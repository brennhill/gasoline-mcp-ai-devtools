---
doc_type: feature_index
feature_id: feature-file-upload
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-03-04
code_paths:
  - cmd/dev-console/tools_interact_upload_handler.go
  - cmd/dev-console/tools_interact_upload.go
  - cmd/dev-console/upload_handlers.go
  - internal/upload/handlers.go
  - internal/upload/security.go
  - internal/upload/os_automation.go
  - scripts/smoke-tests/upload-server.py
test_paths:
  - cmd/dev-console/tools_interact_upload_test.go
  - cmd/dev-console/upload_handlers_test.go
  - internal/upload/security_test.go
  - internal/upload/os_automation_test.go
  - scripts/smoke-tests/test-upload-server.py
  - scripts/smoke-tests/15-file-upload.sh
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
- Flow Map Pointer: [flow-map.md](./flow-map.md)

## Canonical Note
Upload is security-first: path validation and policy checks must pass before any OS-level dialog automation runs.
