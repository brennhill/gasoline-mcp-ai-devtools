// pending-queries.ts — Thin dispatcher shell.
// Delegates to command modules registered in commands/.
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