import type { PendingQuery } from '../types/queries';
import type { SyncClient } from './sync-client';
import type { SendAsyncResultFn, ActionToastFn } from './pending-queries';
interface VerifyResult {
    has_file: boolean;
    file_name?: string;
    file_size?: number;
}
interface ClickResult {
    clicked: boolean;
    error?: string;
}
interface EscalationResult {
    success: boolean;
    stage: number;
    escalation_reason?: string;
    file_name?: string;
    error?: string;
}
/**
 * Verify whether a file is present on the input element after Stage 1 injection.
 * Polls up to VERIFY_MAX_ATTEMPTS times with VERIFY_DELAY_MS between attempts.
 */
export declare function verifyFileOnInput(tabId: number, selector: string): Promise<VerifyResult>;
/**
 * Click a file input element to open the native file dialog.
 */
export declare function clickFileInput(tabId: number, selector: string): Promise<ClickResult>;
/**
 * Escalate to Stage 4 OS automation: click file input, call daemon, verify result.
 */
export declare function escalateToStage4(tabId: number, selector: string, filePath: string, serverUrl: string): Promise<EscalationResult>;
export declare function executeUpload(query: PendingQuery, tabId: number, syncClient: SyncClient, sendAsyncResult: SendAsyncResultFn, actionToast: ActionToastFn): Promise<void>;
export {};
//# sourceMappingURL=upload-handler.d.ts.map