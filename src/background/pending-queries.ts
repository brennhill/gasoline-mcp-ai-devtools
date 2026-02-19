// pending-queries.ts — Thin dispatcher shell.
// Delegates to command modules registered in commands/.

import type { PendingQuery } from '../types'
import type { SyncClient } from './sync-client'
import { dispatch } from './commands/registry'

// Import command modules to trigger handler registration
import './commands/observe'
import './commands/analyze'
import './commands/interact'

// Re-export types for backward compatibility (used by browser-actions.ts, upload-handler.ts, dom-dispatch.ts)
export type { SendAsyncResultFn, ActionToastFn } from './commands/helpers'

// Re-export handlePilotCommand (used by index.ts re-export chain)
export { handlePilotCommand } from './commands/interact'

export async function handlePendingQuery(query: PendingQuery, syncClient: SyncClient): Promise<void> {
  return dispatch(query, syncClient)
}
