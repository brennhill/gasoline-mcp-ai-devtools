/**
 * Purpose: Handles extension background coordination and message routing.
 * Why: Centralizes extension coordination to reduce race conditions and split-brain state.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
import { dispatch } from './commands/registry.js';
// Import command modules to trigger handler registration
import './commands/observe.js';
import './commands/analyze.js';
import './commands/interact.js';
// Re-export handlePilotCommand (used by index.ts re-export chain)
export { handlePilotCommand } from './commands/interact.js';
export async function handlePendingQuery(query, syncClient) {
    return dispatch(query, syncClient);
}
//# sourceMappingURL=pending-queries.js.map