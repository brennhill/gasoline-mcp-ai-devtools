/**
 * Purpose: Shared helper that opens the terminal side panel and requests the tracked-site audit workflow.
 * Why: Keeps popup and hover entrypoints aligned on one audit-trigger contract.
 */
/**
 * @fileoverview request-audit.ts - Shared runtime helper for launching the Kaboom audit workflow.
 */
import { requestWorkspaceAudit } from './workspace-actions.js';
export async function requestAudit(pageUrl) {
    await requestWorkspaceAudit(pageUrl);
}
//# sourceMappingURL=request-audit.js.map