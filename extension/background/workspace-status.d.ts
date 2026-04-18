/**
 * Purpose: Assembles workspace status snapshots for the sidepanel from content heuristics and background session state.
 * Why: Keeps workspace QA state in one typed place instead of duplicating logic across hover, popup, and sidepanel surfaces.
 * Docs: docs/features/feature/terminal/index.md
 */
import type { WorkspaceContentStatusPayload, WorkspaceStatusMode, WorkspaceStatusSnapshot } from '../types/workspace-status.js';
interface WorkspaceStatusTab {
    readonly id?: number;
    readonly title?: string;
    readonly url?: string;
}
interface BuildWorkspaceStatusSnapshotOptions {
    readonly mode: WorkspaceStatusMode;
    readonly tab: WorkspaceStatusTab;
    readonly recordingState?: {
        readonly active?: boolean;
    } | null;
    readonly audit?: {
        readonly updated_at?: string;
    } | null;
    readonly queryContentStatus: () => Promise<WorkspaceContentStatusPayload>;
}
export declare function buildWorkspaceStatusSnapshot(options: BuildWorkspaceStatusSnapshotOptions): Promise<WorkspaceStatusSnapshot>;
export declare function getWorkspaceStatusSnapshot(options?: {
    readonly mode?: WorkspaceStatusMode;
    readonly tabId?: number;
}): Promise<WorkspaceStatusSnapshot>;
export {};
//# sourceMappingURL=workspace-status.d.ts.map