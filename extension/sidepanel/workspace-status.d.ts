/**
 * Purpose: Renders workspace QA summary and status regions from typed snapshots.
 * Why: Keeps sidepanel display logic separate from terminal-session orchestration.
 * Docs: docs/features/feature/terminal/index.md
 */
import type { WorkspaceStatusSnapshot } from '../types/workspace-status.js';
export declare function renderWorkspaceStatus(summaryStripEl: HTMLDivElement, statusAreaEl: HTMLDivElement, snapshot: WorkspaceStatusSnapshot, contextMessage?: string | null): void;
//# sourceMappingURL=workspace-status.d.ts.map