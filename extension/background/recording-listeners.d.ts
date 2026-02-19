/** Dependencies injected by recording.ts to avoid circular imports. */
export interface RecordingListenerDeps {
    startRecording: (name: string, fps: number, queryId: string, audio: string, fromPopup: boolean, targetTabId?: number) => Promise<{
        status: string;
        name: string;
        startTime?: number;
        error?: string;
    }>;
    stopRecording: (truncated?: boolean) => Promise<{
        status: string;
        name: string;
        duration_seconds?: number;
        size_bytes?: number;
        truncated?: boolean;
        path?: string;
        error?: string;
    }>;
    isActive: () => boolean;
    getTabId: () => number;
    setInactive: () => void;
    clearRecordingState: () => Promise<void>;
    getServerUrl: () => string;
}
/**
 * Install all chrome.runtime.onMessage listeners for recording.
 * Must be called once at module load time, guarded by chrome runtime availability.
 */
export declare function installRecordingListeners(deps: RecordingListenerDeps): void;
//# sourceMappingURL=recording-listeners.d.ts.map