/**
 * Purpose: Owns mixed page-context injection policy for the workspace sidepanel.
 * Why: Keeps auto/manual terminal injections and route-refresh behavior separate from shell rendering.
 * Docs: docs/features/feature/terminal/index.md
 */
import type { WorkspaceStatusSnapshot } from '../types/workspace-status.js';
export interface WorkspaceContextUiState {
    readonly message: string | null;
}
interface WorkspaceContextControllerOptions {
    readonly hostTabId?: number;
    readonly writeToTerminal: (text: string) => void;
    readonly shouldDeferWrite: () => boolean;
    readonly onUiStateChange: (state: WorkspaceContextUiState) => void;
    readonly refreshWorkspaceStatus: (mode?: 'live' | 'audit') => Promise<WorkspaceStatusSnapshot | undefined>;
}
export interface WorkspaceContextController {
    setSnapshot: (snapshot: WorkspaceStatusSnapshot) => void;
    handleWorkspaceOpen: (snapshot: WorkspaceStatusSnapshot | undefined) => void;
    handleAuditSnapshot: (snapshot: WorkspaceStatusSnapshot) => void;
    injectCurrentContext: () => void;
    reset: () => void;
    dispose: () => void;
}
export declare function createWorkspaceContextController(options: WorkspaceContextControllerOptions): WorkspaceContextController;
export {};
//# sourceMappingURL=workspace-context.d.ts.map