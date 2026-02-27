/**
 * Purpose: Handles extension background coordination and message routing.
 * Why: Centralizes extension coordination to reduce race conditions and split-brain state.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */

// pending-queries.ts — Thin dispatcher shell.
// Delegates to command modules registered in commands/.

import type { PendingQuery } from '../types/index.js'
import type { SyncClient } from './sync-client.js'
import { dispatch } from './commands/registry.js'

// Import command modules to trigger handler registration
import './commands/observe.js'
import './commands/analyze.js'
import './commands/analyze-navigation.js'
import './commands/analyze-page-structure.js'
import './commands/interact.js'
import './commands/interact-content.js'
import './commands/interact-explore.js'

// Re-export types for backward compatibility (used by browser-actions.ts, upload-handler.ts, dom-dispatch.ts)
export type { SendAsyncResultFn, ActionToastFn } from './commands/helpers.js'

// Re-export handlePilotCommand (used by index.ts re-export chain)
export { handlePilotCommand } from './commands/interact.js'

export async function handlePendingQuery(query: PendingQuery, syncClient: SyncClient): Promise<void> {
  return dispatch(query, syncClient)
}
