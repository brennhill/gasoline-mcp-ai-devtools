/**
 * @fileoverview Version Check - Periodic version checking and badge management
 */
import { isVersionNewer } from '../lib/version.js';
/**
 * Version check state
 */
let lastCheckedAt = 0;
let lastServerVersion = null;
let newVersionAvailable = false;
// Version check interval: 6 hours (in milliseconds)
const VERSION_CHECK_INTERVAL_MS = 6 * 60 * 60 * 1000;
/**
 * Get the extension version from manifest
 */
export function getExtensionVersion() {
    const manifest = chrome.runtime.getManifest();
    return manifest.version || '0.0.0';
}
/**
 * Check if a new version is available (based on last check)
 */
export function isNewVersionAvailable() {
    return newVersionAvailable;
}
/**
 * Get the last checked server version
 */
export function getLastServerVersion() {
    return lastServerVersion;
}
/**
 * Check server version and update state
 * Updates newVersionAvailable state and badge
 */
export async function checkServerVersion(serverUrl, debugLogFn) {
    const now = Date.now();
    // Rate limit: only check every VERSION_CHECK_INTERVAL_MS
    if (now - lastCheckedAt < VERSION_CHECK_INTERVAL_MS) {
        return;
    }
    try {
        const response = await fetch(`${serverUrl}/health`);
        if (!response.ok) {
            if (debugLogFn) {
                debugLogFn('version', `Server health check failed: HTTP ${response.status}`);
            }
            return;
        }
        const data = (await response.json());
        const serverVersion = data.version;
        lastCheckedAt = now;
        if (!serverVersion) {
            if (debugLogFn) {
                debugLogFn('version', 'Server returned no version info');
            }
            return;
        }
        lastServerVersion = serverVersion;
        const extensionVersion = getExtensionVersion();
        const isNewer = isVersionNewer(serverVersion, extensionVersion);
        if (isNewer !== newVersionAvailable) {
            newVersionAvailable = isNewer;
            updateVersionBadge();
            if (debugLogFn) {
                debugLogFn('version', 'Version check result', {
                    extensionVersion,
                    serverVersion,
                    updateAvailable: isNewer,
                });
            }
        }
    }
    catch (error) {
        if (debugLogFn) {
            debugLogFn('version', 'Version check failed', {
                error: error.message,
            });
        }
    }
}
/**
 * Update extension badge to show version update indicator
 * If newVersionAvailable, shows a "⬆" indicator on the icon
 */
export function updateVersionBadge() {
    if (typeof chrome === 'undefined' || !chrome.action)
        return;
    if (newVersionAvailable) {
        chrome.action.setBadgeText({
            text: '⬆',
        });
        chrome.action.setBadgeBackgroundColor({
            color: '#0969da', // Blue for info
        });
        chrome.action.setTitle({
            title: `Gasoline: New version available (${lastServerVersion})`,
        });
    }
    else {
        // Clear the version update indicator
        // Keep error count badge if any
        chrome.action.setTitle({
            title: 'Gasoline',
        });
    }
}
/**
 * Reset version check state (useful for testing)
 */
export function resetVersionCheck() {
    lastCheckedAt = 0;
    lastServerVersion = null;
    newVersionAvailable = false;
}
//# sourceMappingURL=version-check.js.map