/**
 * Purpose: Builds the QA workspace shell that wraps the existing terminal pane.
 * Why: Keeps the sidepanel layout and placeholder workspace chrome separate from terminal session logic.
 * Docs: docs/features/feature/terminal/index.md
 */
export declare const WORKSPACE_SUMMARY_STRIP_ID = "kaboom-workspace-summary-strip";
export declare const WORKSPACE_ACTION_ROW_ID = "kaboom-workspace-action-row";
export declare const WORKSPACE_TERMINAL_REGION_ID = "kaboom-workspace-terminal-region";
export declare const WORKSPACE_STATUS_AREA_ID = "kaboom-workspace-status-area";
export interface WorkspaceShellElements {
    readonly rootEl: HTMLDivElement;
    readonly terminalRegionEl: HTMLDivElement;
    readonly summaryStripEl: HTMLDivElement;
    readonly actionRowEl: HTMLDivElement;
    readonly statusAreaEl: HTMLDivElement;
}
export interface WorkspaceShellActions {
    readonly onToggleRecording: () => void;
    readonly onScreenshot: () => void;
    readonly onRunAudit: () => void;
    readonly onAddNote: () => void;
    readonly onInjectContext: () => void;
    readonly onResetWorkspace: () => void;
}
export declare function createWorkspaceShell(terminalPaneEl: HTMLDivElement, actions: WorkspaceShellActions): WorkspaceShellElements;
//# sourceMappingURL=workspace-shell.d.ts.map