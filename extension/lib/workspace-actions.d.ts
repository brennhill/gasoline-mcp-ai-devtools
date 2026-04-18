/**
 * Purpose: Centralizes workspace-related runtime and tab action helpers shared by popup, hover, and sidepanel UI.
 * Why: Keeps QA action entrypoints aligned so audit, screenshot, and note capture do not drift across surfaces.
 * Docs: docs/features/feature/terminal/index.md
 */
export declare function openWorkspace(): Promise<void>;
export declare function requestWorkspaceAudit(pageUrl?: string): Promise<void>;
export declare function requestWorkspaceScreenshot(): Promise<unknown>;
export declare function requestWorkspaceNoteMode(tabId: number): Promise<unknown>;
export declare function toggleWorkspaceRecording(recordingActive: boolean): Promise<unknown>;
//# sourceMappingURL=workspace-actions.d.ts.map