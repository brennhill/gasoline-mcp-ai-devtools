export declare const ALARM_NAME_ANALYTICS = "analyticsPing";
export interface DailyFlags {
    ai_connected: boolean;
    screenshot: boolean;
    js_exec: boolean;
    annotations: boolean;
    video: boolean;
    dom_action: boolean;
    a11y: boolean;
    network_observe: boolean;
}
/**
 * Called from command dispatch to record that a feature was used today.
 * Cheap — sets a boolean in memory, no async work.
 */
export declare function trackCommandUsage(commandType: string): void;
/**
 * Called when AI/MCP connection is established.
 */
export declare function trackAiConnected(): void;
/**
 * Initialize analytics on extension startup.
 * Loads persisted flags, sets up alarm, sends initial ping if needed.
 */
export declare function initAnalytics(): Promise<void>;
/**
 * Handle the analytics alarm firing. Called from alarm listener.
 */
export declare function handleAnalyticsAlarm(): Promise<void>;
//# sourceMappingURL=analytics.d.ts.map