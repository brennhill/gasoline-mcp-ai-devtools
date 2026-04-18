/**
 * Purpose: Builds the terminal pane used inside the workspace sidebar shell.
 * Why: Keeps the xterm host, header controls, and iframe mounting isolated from workspace chrome.
 * Docs: docs/features/feature/terminal/index.md
 */
export interface CreateWorkspaceTerminalPaneOptions {
    readonly token: string;
    readonly serverUrl: string;
    readonly onDisconnect: (event: MouseEvent) => void;
    readonly onRedraw: (event: MouseEvent) => void;
    readonly onMinimize: (event: MouseEvent) => void;
}
export interface WorkspaceTerminalPaneElements {
    readonly shellEl: HTMLDivElement;
    readonly bodyEl: HTMLDivElement;
    readonly iframeEl: HTMLIFrameElement | null;
    readonly statusDotEl: HTMLSpanElement;
    readonly minimizeButtonEl: HTMLButtonElement;
}
export declare function createWorkspaceTerminalPane(options: CreateWorkspaceTerminalPaneOptions): WorkspaceTerminalPaneElements;
//# sourceMappingURL=workspace-terminal-pane.d.ts.map