# Modular Architecture Principal Review (2026-02-27)

Date: 2026-02-27  
Branch: `modular-architecture`  
Scope: extension background runtime + `/sync` transport + server tool dispatch

## Goal

Design a fully modular architecture where:

1. Transport can be swapped (localhost HTTP now, WebSocket or other provider later).
2. Every executable capability is registered in a feature registry.
3. Server and extension exchange supported features and detect mismatches.
4. Meta-features declare dependencies and are resolved through dependency injection.
5. New features can be added by creating a module, not by editing global switch chains.

## Decisions Locked (from review Q&A, 2026-02-27)

1. Unknown/missing features: warning by default; optional strict mode at runtime.
2. Compatibility model: per-feature semver compatibility (major-compatible baseline, feature-level negotiation allowed).
3. Provider scope: all extension-server traffic, not only `/sync`.
4. Registry model: separate server and extension registries; overlap negotiated over transport.
5. Meta-feature orchestration: extension-side feature composition using declared dependencies.
6. Registration mode: static registration is acceptable.
7. Compatibility policy: no backward-compatibility layer required for legacy params/aliases.
8. Strict mode scope: runtime toggle (not environment-scoped).

## Current Baseline (Observed)

1. Extension already has command registry dispatch (`src/background/commands/registry.ts`) and command modules registered via side-effect imports (`src/background/pending-queries.ts`).
2. Extension `/sync` loop is centralized in `src/background/sync-client.ts` and coordinated by `src/background/sync-manager.ts`.
3. Extension transport is HTTP-coupled (`src/background/server.ts` + `src/background/communication.ts`).
4. Server already has top-level tool module registry (`cmd/dev-console/tools_registry.go`) but per-tool mode/action dispatch is still map/switch driven in tool files.
5. `/sync` contract exists in `internal/capture/sync.go` and is the canonical command transport.
6. Mode capability metadata exists for MCP clients (`internal/tools/configure/capabilities.go`) but not as executable feature negotiation between server and extension.

## Target Architecture

### 1) Two Explicit Module Registries

Create two first-class registries:

1. `FeatureRegistry` (extension runtime features)
2. `ToolModeRegistry` (server MCP tool modes and query factories)

Each registry entry must include:

1. `id` (stable canonical identifier)
2. `version` (semver for compatibility)
3. `kind` (`feature`, `meta_feature`, `tool_mode`)
4. `depends_on` (array of feature IDs)
5. `provides` (capability tags, optional)
6. `owner` (`extension` or `server`)
7. `register()` / `execute()` hooks

### 2) Provider Abstraction (Transport Layer, All Traffic)

Introduce a shared transport concept on both sides:

1. TypeScript: `ExtensionTransportProvider`
2. Go: `ExtensionTransportProvider`

Transport implementations should mirror each other by transport ID:

1. `HTTPExtensionTransportProvider` (TS + Go)
2. `WebSocketExtensionTransportProvider` (TS + Go, future)

Suggested extension-side interface:

```ts
interface ExtensionTransportProvider {
  id(): string
  start(): void
  stop(): void
  flush(): void
  setEndpoint(endpoint: string): void
  getState(): ProviderState
  sendSync(request: SyncEnvelope): Promise<SyncEnvelopeResponse>
  postLogs(entries: LogEntry[]): Promise<void>
  postWebSocketEvents(events: WebSocketEvent[]): Promise<void>
  postNetworkBodies(bodies: NetworkBodyPayload[]): Promise<void>
  postNetworkWaterfall(payload: WaterfallPayload): Promise<void>
  postEnhancedActions(actions: EnhancedAction[]): Promise<void>
  postPerformanceSnapshots(snapshots: PerformanceSnapshot[]): Promise<void>
  postScreenshot(payload: ScreenshotPayload): Promise<ScreenshotResult>
  checkHealth(): Promise<ServerHealthResponse>
}
```

First implementation:

1. `HTTPExtensionTransportProvider` (wrap current HTTP endpoints + `/sync`)

Planned implementations:

1. `WebSocketExtensionTransportProvider`
2. `RemoteAgentExtensionTransportProvider` (non-localhost endpoint profiles)

Rule: feature modules must not call `fetch` directly for server communication; they depend on provider interfaces from runtime context.

### 3) Sync-Time Feature Negotiation

Extend `/sync` payloads with negotiated manifests:

1. Extension sends `extension_features` and `provider_id`.
2. Server responds with `server_features`.
3. Optional: both sides send `profile_id` (for subset deployments, e.g., full extension vs remote-web subset).
3. Both sides compute and log:
   1. `unknown_to_server` (extension-only features)
   2. `unknown_to_extension` (server-only features)
   3. `version_incompatible` (same ID, incompatible version range)

Minimum behavior:

1. Unknown features are warnings, not hard failures (default mode).
2. Strict mode (runtime toggle) can reject dispatch of unsupported features.

### 4) Dependency Injection and Meta-Features

Every feature receives a narrow runtime dependency bag:

```ts
interface FeatureDeps {
  provider: ExtensionTransportProvider
  tabResolver: TabResolver
  domExecutor: DomExecutor
  screenshotService: ScreenshotService
  logger: Logger
  clock: Clock
}
```

Meta-feature modules define explicit dependencies:

1. Example: `interact.navigate_with_screenshot`
2. `depends_on = ["interact.navigate", "observe.screenshot"]`
3. Runtime ensures dependency graph is valid and acyclic at startup.

No module may import global mutable state directly from `index.ts`.

### 5) Segmented Feature File Layout

Proposed extension layout:

1. `src/background/runtime/`  
   Registry, bootstrapping, DI container, lifecycle.
2. `src/background/providers/`  
   `http-sync-provider.ts`, `websocket-sync-provider.ts`.
3. `src/background/features/<feature-id>/`  
   `descriptor.ts`, `handler.ts`, `types.ts`, `test.ts`.
4. `src/background/meta-features/<meta-id>/`  
   Orchestration-only modules with declared `depends_on`.

Proposed server layout additions:

1. `internal/features/registry/`  
   Registry primitives, dependency resolver, compatibility checks.
2. `internal/features/server/`  
   Tool-mode modules mapped to query factory + result encoder.
3. `internal/features/shared/`  
   Feature IDs and compatibility helpers shared by server modules.

## Canonical ID Model

Use stable IDs independent of file names:

1. `observe.screenshot`
2. `observe.page_inventory`
3. `interact.navigate`
4. `interact.dom.click`
5. `analyze.accessibility`
6. `meta.navigate_with_screenshot`

No legacy aliases: only canonical IDs are accepted in the modular architecture cutover.

## Dispatch Flow (Target)

1. MCP call enters server tool module.
2. Tool mode resolves to canonical feature ID(s).
3. Server checks extension negotiated feature support.
4. Server enqueues query with feature ID in envelope metadata.
5. Extension registry resolves handler by feature ID.
6. Handler executes with injected dependencies.
7. Result includes canonical feature ID for tracing and reconciliation.

## Migration Plan (Low Risk)

### Phase 0: Contract Guardrails

1. Add parity tests: schema enums vs registry IDs vs dispatch maps.
2. Add startup tests that fail on duplicate feature IDs or dependency cycles.

### Phase 1: Registry Metadata Only

1. Keep existing runtime behavior.
2. Add descriptors for current features and tool modes.
3. Add diagnostics command to dump runtime registry state.

### Phase 2: Provider Seam (No Behavior Change)

1. Wrap current HTTP calls in `HTTPExtensionTransportProvider`.
2. Route sync manager and telemetry/screenshot/upload/health calls through provider interface.

### Phase 3: Feature Negotiation over `/sync`

1. Add optional feature manifest fields to request/response.
2. Log unknown/mismatch features.
3. Keep warning-only behavior by default.

### Phase 4: DI + Module Isolation

1. Move feature logic into segmented folders.
2. Pass dependency bags instead of importing global state.

### Phase 5: Strict Capability Enforcement (Optional)

1. Add runtime toggle for strict mode.
2. Block unsupported feature dispatch in strict mode.

## Testing Requirements

1. Registry integrity tests:
   1. Unique IDs
   2. Acyclic dependencies
   3. Declared dependencies exist
2. Negotiation tests:
   1. Unknown feature warnings
   2. Version mismatch warnings
   3. Strict-mode rejection behavior
3. Provider conformance tests:
   1. Same command/result semantics across providers
   2. Reconnect behavior and flush semantics
4. End-to-end tests:
   1. MCP -> server registry -> `/sync` -> extension registry -> result
   2. Meta-feature execution with dependency resolution

## Recommended First Implementation Slice

Implement a narrow vertical slice:

1. Add `FeatureRegistry` + `HttpSyncProvider` wrappers.
2. Convert two features:
   1. `interact.navigate`
   2. `observe.screenshot`
3. Add one meta-feature:
   1. `meta.navigate_with_screenshot`
4. Add `/sync` feature manifest exchange as warning-only.

This slice validates the architecture without requiring a full rewrite.

## Strict Toggle Recommendation

Requested model (single runtime toggle) is valid.

One optional refinement worth considering:

1. Store strict mode by extension session ID, not process-global, if you expect mixed-version clients/providers concurrently.
2. If deployment is single-client only, a process-global runtime toggle is simpler and fully acceptable.
