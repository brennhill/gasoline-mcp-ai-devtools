/**
 * @fileoverview Version Check - Periodic version checking and badge management
 */
/**
 * Get the extension version from manifest
 */
export declare function getExtensionVersion(): string;
/**
 * Check if a new version is available (based on last check)
 */
export declare function isNewVersionAvailable(): boolean;
/**
 * Get the last checked server version
 */
export declare function getLastServerVersion(): string | null;
/**
 * Check server version and update state
 * Updates newVersionAvailable state and badge
 */
export declare function checkServerVersion(serverUrl: string, debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<void>;
/**
 * Update extension badge to show version update indicator
 * If newVersionAvailable, shows a "⬆" indicator on the icon
 */
export declare function updateVersionBadge(): void;
/**
 * Reset version check state (useful for testing)
 */
export declare function resetVersionCheck(): void;
//# sourceMappingURL=version-check.d.ts.map