/**
 * Purpose: Provides backward-compatible message/type export surface for extension communication contracts.
 * Why: Preserves existing imports while message definitions are split into focused type modules.
 * Docs: docs/features/feature/query-service/index.md
 */
/**
 * @fileoverview Message Types for Gasoline Extension
 *
 * Comprehensive discriminated unions for all message types used in the extension.
 * This is the single source of truth for message payloads between:
 * - Background service worker
 * - Content scripts
 * - Inject scripts (page context)
 * - Popup
 *
 * NOTE: This file now re-exports types from focused modules for backward compatibility.
 * New code should import from the specific modules directly.
 */
export * from './telemetry';
export * from './websocket';
export * from './network';
export * from './performance';
export * from './actions';
export * from './ai-context';
export * from './accessibility';
export * from './dom';
export * from './state';
export * from './queries';
export * from './sourcemap';
export * from './chrome';
export * from './debug';
export * from './runtime-messages';
//# sourceMappingURL=messages.d.ts.map