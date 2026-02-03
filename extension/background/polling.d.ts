/**
 * @fileoverview Polling - Handles all polling loops including query polling,
 * settings heartbeat, waterfall posting, extension logs, and status pings.
 */
/**
 * Start polling for pending queries at 1-second intervals
 */
export declare function startQueryPolling(
  pollFn: () => Promise<void>,
  debugLogFn?: (category: string, message: string, data?: unknown) => void,
): void
/**
 * Stop polling for pending queries
 */
export declare function stopQueryPolling(): void
/**
 * Check if query polling is active
 */
export declare function isQueryPollingActive(): boolean
/**
 * Start settings heartbeat: POST /settings every 2 seconds
 */
export declare function startSettingsHeartbeat(
  postSettingsFn: () => Promise<void>,
  debugLogFn?: (category: string, message: string, data?: unknown) => void,
): void
/**
 * Stop settings heartbeat
 */
export declare function stopSettingsHeartbeat(
  debugLogFn?: (category: string, message: string, data?: unknown) => void,
): void
/**
 * Check if settings heartbeat is active
 */
export declare function isSettingsHeartbeatActive(): boolean
/**
 * Start network waterfall posting: POST /network-waterfall every 10 seconds
 */
export declare function startWaterfallPosting(
  postWaterfallFn: () => Promise<void>,
  debugLogFn?: (category: string, message: string, data?: unknown) => void,
): void
/**
 * Stop network waterfall posting
 */
export declare function stopWaterfallPosting(
  debugLogFn?: (category: string, message: string, data?: unknown) => void,
): void
/**
 * Check if waterfall posting is active
 */
export declare function isWaterfallPostingActive(): boolean
/**
 * Start extension logs posting: POST /extension-logs every 5 seconds
 */
export declare function startExtensionLogsPosting(postLogsFn: () => Promise<void>): void
/**
 * Stop extension logs posting
 */
export declare function stopExtensionLogsPosting(): void
/**
 * Check if extension logs posting is active
 */
export declare function isExtensionLogsPostingActive(): boolean
/**
 * Start status ping: POST /api/extension-status every 30 seconds
 */
export declare function startStatusPing(sendPingFn: () => Promise<void>): void
/**
 * Stop status ping
 */
export declare function stopStatusPing(): void
/**
 * Check if status ping is active
 */
export declare function isStatusPingActive(): boolean
/**
 * Start version check: check GitHub for new releases (once daily)
 */
export declare function startVersionCheck(
  checkVersionFn: () => Promise<void>,
  debugLogFn?: (category: string, message: string, data?: unknown) => void,
): void
/**
 * Stop version check
 */
export declare function stopVersionCheck(debugLogFn?: (category: string, message: string, data?: unknown) => void): void
/**
 * Check if version check is active
 */
export declare function isVersionCheckActive(): boolean
/**
 * Stop all polling loops
 */
export declare function stopAllPolling(): void
//# sourceMappingURL=polling.d.ts.map
