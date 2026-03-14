/**
 * Purpose: Pure recording UI state rendering — recording/idle/error/notice display functions.
 * Why: Separates stateless UI rendering from the complex setup/wiring in recording.ts.
 * Docs: docs/features/feature/playback-engine/index.md
 * Docs: docs/features/feature/tab-recording/index.md
 */
import type { RecordingElements, RecordingState } from './recording-io.js';
interface PendingRecordingIntent {
    highlight?: boolean;
    name?: string;
    fps?: number;
    audio?: string;
    tabId?: number;
    url?: string;
}
interface ApprovalElements {
    card: HTMLElement | null;
    detail: HTMLElement | null;
    approveBtn: HTMLButtonElement | null;
    denyBtn: HTMLButtonElement | null;
}
export declare function getRecordSection(els: RecordingElements): Element | null;
export declare function applyRecordHighlight(els: RecordingElements): void;
export declare function removeRecordHighlight(els: RecordingElements): void;
export declare function showRecording(els: RecordingElements, state: RecordingState, name: string, startTime: number): void;
export declare function showIdle(els: RecordingElements, state: RecordingState): void;
export declare function describePendingRecording(pending: PendingRecordingIntent): string;
export declare function setApprovalPendingState(els: RecordingElements, approvalEls: ApprovalElements, state: RecordingState, pending: PendingRecordingIntent | null): void;
export declare function showTopNotice(els: RecordingElements, text: string): void;
export declare function showSavedLink(saveInfoEl: HTMLElement, displayName: string, filePath: string): void;
export declare function showSaveResult(saveInfoEl: HTMLElement | null, resp: {
    status?: string;
    name?: string;
    path?: string;
    error?: string;
} | undefined): void;
export declare function showStartError(saveInfoEl: HTMLElement | null, errorText: string): void;
export type { PendingRecordingIntent, ApprovalElements };
//# sourceMappingURL=recording-ui-state.d.ts.map