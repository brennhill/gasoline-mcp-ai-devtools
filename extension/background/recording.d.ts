/** Returns whether a recording is currently active. */
export declare function isRecording(): boolean;
/** Returns current recording info for popup sync. */
export declare function getRecordingInfo(): {
    active: boolean;
    name: string;
    startTime: number;
};
/**
 * Start recording the active tab.
 * @param name — Pre-generated filename from the Go server (e.g., "checkout-bug--2026-02-07-1423")
 * @param fps — Framerate (5–60, default 15)
 * @param queryId — PendingQuery ID for result resolution
 */
export declare function startRecording(name: string, fps?: number, queryId?: string): Promise<{
    status: string;
    name: string;
    error?: string;
}>;
/**
 * Stop recording and save the video.
 * @param truncated — true if auto-stopped due to memory guard or tab close
 */
export declare function stopRecording(truncated?: boolean): Promise<{
    status: string;
    name: string;
    duration_seconds?: number;
    size_bytes?: number;
    truncated?: boolean;
    error?: string;
}>;
//# sourceMappingURL=recording.d.ts.map