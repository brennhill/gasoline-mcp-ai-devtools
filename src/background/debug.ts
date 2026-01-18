/**
 * @fileoverview Debug Logging Utilities
 * Standalone module to avoid circular dependencies.
 */

/** Log categories for debug output */
export const DebugCategory = {
  CONNECTION: 'connection' as const,
  CAPTURE: 'capture' as const,
  ERROR: 'error' as const,
  LIFECYCLE: 'lifecycle' as const,
  SETTINGS: 'settings' as const,
  SOURCEMAP: 'sourcemap' as const,
  QUERY: 'query' as const,
}

export type DebugCategoryType = (typeof DebugCategory)[keyof typeof DebugCategory]
