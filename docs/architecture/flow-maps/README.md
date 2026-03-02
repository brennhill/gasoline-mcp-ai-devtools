---
doc_type: flow_map_index
status: active
last_reviewed: 2026-03-02
owners:
  - Brenn
---

# Flow Maps

Flow maps are concise architecture navigation docs optimized for both human maintainers and LLM retrieval.

## Placement Best Practice

Use a hybrid placement model:

1. Keep canonical flow maps in `docs/architecture/flow-maps/`.
2. Add feature-local `flow-map.md` pointer files in relevant feature directories.
3. Link each feature `index.md` to its local pointer file.

This avoids content drift while preserving high retrieval recall.

## Format Standard

Each flow map should include:

1. Scope
2. Entrypoints
3. Primary Flow (numbered)
4. Error and Recovery Paths
5. State and Contracts
6. Code Paths
7. Test Paths
8. Edit Guardrails

## Current Maps

- [Bridge Startup Contention and Convergence](./bridge-startup-contention-and-convergence.md)
- [Analyze Annotation Waiter and Flush Recovery](./analyze-annotations-waiter-and-flush.md)
- [Daemon Stop and Force Cleanup](./daemon-stop-and-force-cleanup.md)
- [DOM Selector Resolution and Disambiguation](./dom-selector-resolution-and-disambiguation.md)
- [MCP Daemon Lifecycle](./mcp-daemon-lifecycle.md)
- [Network Recording Control](./network-recording-control.md)
- [Observe Dispatch and Augmentation](./observe-dispatch-and-augmentation.md)
- [Playbook Resource Resolution](./playbook-resource-resolution.md)
- [Recording Control and Playback](./recording-control-and-playback.md)
- [Tab Recording and Media Ingest](./tab-recording-and-media-ingest.md)
- [Self-Testing Test Harness](./self-testing-test-harness.md)
- [Test Generation Dispatch](./test-generation-dispatch.md)
- [Issue Reporting Submission](./issue-reporting-submission.md)
