/**
 * Purpose: DOM subsystem barrel — types, primitives, dispatch, and frame targeting for browser DOM interaction.
 * Why: Groups all DOM-related functionality into a cohesive module with a single public API surface.
 */

// Types
export type {
  DOMMutationEntry,
  ScopeRect,
  BoundingBox,
  DOMResult,
  DOMPrimitiveOptions,
  DOMActionParams
} from './types.js'

// Action metadata
export { ACTION_METADATA, isReadOnlyAction, isMutatingAction } from './action-metadata.js'
export type { ActionMeta } from './action-metadata.js'

// Frame targeting
export {
  INVALID_FRAME_ERROR,
  FRAME_NOT_FOUND_ERROR,
  normalizeFrameArg,
  resolveMatchedFrameIds
} from './frame-targeting.js'
export type { NormalizedFrameTarget } from './frame-targeting.js'

// Primitives
export { domPrimitive, domWaitFor } from './primitives.js'
export { domPrimitiveListInteractive } from './primitives-list-interactive.js'
export { domPrimitiveNavDiscovery } from './primitives-nav-discovery.js'
export { domPrimitiveOverlay } from './primitives-overlay.js'
export { domPrimitiveQuery } from './primitives-query.js'
export { domPrimitiveWaitForStable, domPrimitiveActionDiff } from './primitives-stability.js'
export { domPrimitiveIntent } from './primitives-intent.js'
export { domFrameProbe } from './frame-probe.js'

// Dispatch
export { executeDOMAction } from './dispatch.js'

// Result reconciliation
export {
  toDOMResult,
  hasMatchedTargetEvidence,
  pickFrameResult,
  mergeListInteractive,
  reconcileDOMLifecycle
} from './result-reconcile.js'
