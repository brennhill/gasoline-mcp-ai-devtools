/**
 * Purpose: Collects deterministic workspace status heuristics from page state in the content script.
 * Why: Provides lightweight QA signals without requiring a full explicit audit on every workspace open.
 * Docs: docs/features/feature/terminal/index.md
 */
import type { WorkspaceContentStatusPayload, WorkspaceStatusHeuristicInput } from '../types/workspace-status.js';
export declare function collectWorkspaceStatusHeuristics(input: WorkspaceStatusHeuristicInput): WorkspaceContentStatusPayload;
export declare function handleWorkspaceStatusQuery(sendResponse: (result: WorkspaceContentStatusPayload | {
    error: string;
    message: string;
}) => void): boolean;
//# sourceMappingURL=workspace-status.d.ts.map