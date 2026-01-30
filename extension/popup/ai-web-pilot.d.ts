/**
 * @fileoverview AI Web Pilot Toggle Module
 * Manages the AI Web Pilot feature toggle
 */
/**
 * Initialize the AI Web Pilot toggle.
 * Read the current state from chrome.storage.local.
 */
export declare function initAiWebPilotToggle(): Promise<void>;
/**
 * Handle AI Web Pilot toggle change.
 *
 * CRITICAL: ONLY background.js updates the state via setAiWebPilotEnabled message.
 * Popup NEVER writes to chrome.storage directly.
 *
 * This ensures single source of truth. If popup wrote to storage directly:
 * 1. Popup updates storage
 * 2. Background cache doesn't update (no listener yet)
 * 3. Pilot command checks cache and gets wrong value
 * 4. User sees toggle "on" but commands fail saying "off"
 *
 * By routing through background, we guarantee:
 * 1. Popup sends message to background
 * 2. Background updates cache immediately
 * 3. Background writes to storage
 * 4. Pilot commands see correct cache state
 * 5. Everything is consistent
 */
export declare function handleAiWebPilotToggle(enabled: boolean): Promise<void>;
//# sourceMappingURL=ai-web-pilot.d.ts.map