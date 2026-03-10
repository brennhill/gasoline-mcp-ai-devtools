/**
 * Purpose: CDP (Chrome DevTools Protocol) subsystem barrel — dispatch, element resolution, and key mappings.
 * Why: Groups all CDP-related functionality into a cohesive module.
 */

// Dispatch
export { isCDPEscalatable, tryCDPEscalation, executeCDPAction } from './dispatch.js'

// Element resolution
export { resolveElement, buildCDPResult } from './element-resolve.js'
export type { ResolvedElement } from './element-resolve.js'

// Key mappings
export { KEY_CODES, charToKeyInfo } from './key-mappings.js'
