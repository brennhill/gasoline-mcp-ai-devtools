/**
 * @fileoverview Gasoline API - Exposes window.__gasoline interface for developers
 * to interact with Gasoline capture capabilities.
 */
import { setContextAnnotation, removeContextAnnotation, clearContextAnnotations, getContextAnnotations, } from '../lib/context.js';
import { computeSelectors, recordEnhancedAction, getEnhancedActionBuffer, clearEnhancedActionBuffer, generatePlaywrightScript, } from '../lib/reproduction.js';
import { getActionBuffer, clearActionBuffer, setActionCaptureEnabled, } from '../lib/actions.js';
import { getNetworkWaterfall, setNetworkWaterfallEnabled, } from '../lib/network.js';
import { getPerformanceMarks, getPerformanceMeasures, setPerformanceMarksEnabled, } from '../lib/performance.js';
import { enrichErrorWithAiContext, setAiContextEnabled, setAiContextStateSnapshot } from '../lib/ai-context.js';
/**
 * Install the window.__gasoline API for developers to interact with Gasoline
 */
export function installGasolineAPI() {
    if (typeof window === 'undefined')
        return;
    window.__gasoline = {
        /**
         * Add a context annotation that will be included with errors
         * @param key - Annotation key (e.g., 'checkout-flow', 'user')
         * @param value - Annotation value
         * @example
         * window.__gasoline.annotate('checkout-flow', { step: 'payment', items: 3 })
         */
        annotate(key, value) {
            return setContextAnnotation(key, value);
        },
        /**
         * Remove a context annotation
         * @param key - Annotation key to remove
         */
        removeAnnotation(key) {
            return removeContextAnnotation(key);
        },
        /**
         * Clear all context annotations
         */
        clearAnnotations() {
            clearContextAnnotations();
        },
        /**
         * Get current context annotations
         * @returns Current annotations or null if none
         */
        getContext() {
            return getContextAnnotations();
        },
        /**
         * Get the user action replay buffer
         * @returns Recent user actions
         */
        getActions() {
            return getActionBuffer();
        },
        /**
         * Clear the user action replay buffer
         */
        clearActions() {
            clearActionBuffer();
        },
        /**
         * Enable or disable action capture
         * @param enabled - Whether to capture user actions
         */
        setActionCapture(enabled) {
            setActionCaptureEnabled(enabled);
        },
        /**
         * Enable or disable network waterfall capture
         * @param enabled - Whether to capture network waterfall
         */
        setNetworkWaterfall(enabled) {
            setNetworkWaterfallEnabled(enabled);
        },
        /**
         * Get current network waterfall
         * @param options - Filter options
         * @returns Network waterfall entries
         */
        getNetworkWaterfall(options) {
            return getNetworkWaterfall(options);
        },
        /**
         * Enable or disable performance marks capture
         * @param enabled - Whether to capture performance marks
         */
        setPerformanceMarks(enabled) {
            setPerformanceMarksEnabled(enabled);
        },
        /**
         * Get performance marks
         * @param options - Filter options
         * @returns Performance mark entries
         */
        getMarks(options) {
            return getPerformanceMarks(options);
        },
        /**
         * Get performance measures
         * @param options - Filter options
         * @returns Performance measure entries
         */
        getMeasures(options) {
            return getPerformanceMeasures(options);
        },
        // === AI Context ===
        /**
         * Enrich an error entry with AI context
         * @param error - Error entry to enrich
         * @returns Enriched error entry
         */
        enrichError(error) {
            return enrichErrorWithAiContext(error);
        },
        /**
         * Enable or disable AI context enrichment
         * @param enabled
         */
        setAiContext(enabled) {
            setAiContextEnabled(enabled);
        },
        /**
         * Enable or disable state snapshot in AI context
         * @param enabled
         */
        setStateSnapshot(enabled) {
            setAiContextStateSnapshot(enabled);
        },
        // === Reproduction Scripts ===
        /**
         * Record an enhanced action (for testing)
         * @param type - Action type
         * @param element - Target element
         * @param opts - Options
         */
        recordAction(type, element, opts) {
            recordEnhancedAction(type, element, opts);
        },
        /**
         * Get the enhanced action buffer
         * @returns
         */
        getEnhancedActions() {
            return getEnhancedActionBuffer();
        },
        /**
         * Clear the enhanced action buffer
         */
        clearEnhancedActions() {
            clearEnhancedActionBuffer();
        },
        /**
         * Generate a Playwright reproduction script
         * @param actions - Actions to convert
         * @param opts - Generation options
         * @returns Playwright test script
         */
        generateScript(actions, opts) {
            return generatePlaywrightScript((actions || getEnhancedActionBuffer()), opts);
        },
        /**
         * Compute multi-strategy selectors for an element
         * @param element
         * @returns
         */
        getSelectors(element) {
            return computeSelectors(element);
        },
        /**
         * Version of the Gasoline API
         */
        version: '5.2.0',
    };
}
/**
 * Uninstall the window.__gasoline API
 */
export function uninstallGasolineAPI() {
    if (typeof window !== 'undefined' && window.__gasoline) {
        delete window.__gasoline;
    }
}
//# sourceMappingURL=api.js.map