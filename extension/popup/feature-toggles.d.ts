/**
 * @fileoverview Feature Toggles Module
 * Manages feature toggle configuration and initialization
 */
import type { FeatureToggleConfig } from './types';
/**
 * Feature toggle configuration
 */
export declare const FEATURE_TOGGLES: readonly FeatureToggleConfig[];
/**
 * Handle feature toggle change
 * CRITICAL ARCHITECTURE: Popup NEVER writes storage directly.
 * It ONLY sends a message to background, which is the single writer.
 * This prevents desynchronization bugs where UI state diverges from actual state.
 */
export declare function handleFeatureToggle(storageKey: string, messageType: string, enabled: boolean): void;
/**
 * Initialize all feature toggles
 */
export declare function initFeatureToggles(): Promise<void>;
//# sourceMappingURL=feature-toggles.d.ts.map