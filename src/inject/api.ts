/**
 * @fileoverview Gasoline API - Exposes window.__gasoline interface for developers
 * to interact with Gasoline capture capabilities.
 */

import type {
  LogEntry,
  ActionEntry,
  EnhancedAction,
  SelectorStrategies,
  WaterfallEntry,
  PerformanceMark,
  PerformanceMeasure,
} from '../types/index';

import {
  setContextAnnotation,
  removeContextAnnotation,
  clearContextAnnotations,
  getContextAnnotations,
} from '../lib/context';
import {
  computeSelectors,
  recordEnhancedAction,
  getEnhancedActionBuffer,
  clearEnhancedActionBuffer,
  generatePlaywrightScript,
} from '../lib/reproduction';
import {
  getActionBuffer,
  clearActionBuffer,
  setActionCaptureEnabled,
} from '../lib/actions';
import {
  getNetworkWaterfall,
  setNetworkWaterfallEnabled,
} from '../lib/network';
import {
  getPerformanceMarks,
  getPerformanceMeasures,
  setPerformanceMarksEnabled,
} from '../lib/performance';
import { enrichErrorWithAiContext, setAiContextEnabled, setAiContextStateSnapshot } from '../lib/ai-context';

/**
 * GasolineAPI interface exposed on window.__gasoline
 */
export interface GasolineAPI {
  annotate(key: string, value: unknown): void;
  removeAnnotation(key: string): void;
  clearAnnotations(): void;
  getContext(): Record<string, unknown> | null;
  getActions(): ActionEntry[];
  clearActions(): void;
  setActionCapture(enabled: boolean): void;
  setNetworkWaterfall(enabled: boolean): void;
  getNetworkWaterfall(options?: { since?: number; initiatorTypes?: string[] }): WaterfallEntry[];
  setPerformanceMarks(enabled: boolean): void;
  getMarks(options?: { since?: number }): PerformanceMark[];
  getMeasures(options?: { since?: number }): PerformanceMeasure[];
  enrichError(error: LogEntry): Promise<LogEntry>;
  setAiContext(enabled: boolean): void;
  setStateSnapshot(enabled: boolean): void;
  recordAction(type: string, element: Element, opts?: Record<string, unknown>): void;
  getEnhancedActions(): EnhancedAction[];
  clearEnhancedActions(): void;
  generateScript(actions?: EnhancedAction[], opts?: Record<string, unknown>): string;
  getSelectors(element: Element): SelectorStrategies;
  setInputValue(selector: string, value: string | boolean): boolean;
  version: string;
}

// Extend Window interface for __gasoline
declare global {
  interface Window {
    __gasoline?: GasolineAPI;
  }
}

/**
 * Install the window.__gasoline API for developers to interact with Gasoline
 */
export function installGasolineAPI(): void {
  if (typeof window === 'undefined') return;

  window.__gasoline = {
    /**
     * Add a context annotation that will be included with errors
     * @param key - Annotation key (e.g., 'checkout-flow', 'user')
     * @param value - Annotation value
     * @example
     * window.__gasoline.annotate('checkout-flow', { step: 'payment', items: 3 })
     */
    annotate(key: string, value: unknown): boolean {
      return setContextAnnotation(key, value);
    },

    /**
     * Remove a context annotation
     * @param key - Annotation key to remove
     */
    removeAnnotation(key: string): boolean {
      return removeContextAnnotation(key);
    },

    /**
     * Clear all context annotations
     */
    clearAnnotations(): void {
      clearContextAnnotations();
    },

    /**
     * Get current context annotations
     * @returns Current annotations or null if none
     */
    getContext(): Record<string, unknown> | null {
      return getContextAnnotations();
    },

    /**
     * Get the user action replay buffer
     * @returns Recent user actions
     */
    getActions(): ActionEntry[] {
      return getActionBuffer() as unknown as ActionEntry[];
    },

    /**
     * Clear the user action replay buffer
     */
    clearActions(): void {
      clearActionBuffer();
    },

    /**
     * Enable or disable action capture
     * @param enabled - Whether to capture user actions
     */
    setActionCapture(enabled: boolean): void {
      setActionCaptureEnabled(enabled);
    },

    /**
     * Enable or disable network waterfall capture
     * @param enabled - Whether to capture network waterfall
     */
    setNetworkWaterfall(enabled: boolean): void {
      setNetworkWaterfallEnabled(enabled);
    },

    /**
     * Get current network waterfall
     * @param options - Filter options
     * @returns Network waterfall entries
     */
    getNetworkWaterfall(options?: { since?: number; initiatorTypes?: string[] }): WaterfallEntry[] {
      return getNetworkWaterfall(options);
    },

    /**
     * Enable or disable performance marks capture
     * @param enabled - Whether to capture performance marks
     */
    setPerformanceMarks(enabled: boolean): void {
      setPerformanceMarksEnabled(enabled);
    },

    /**
     * Get performance marks
     * @param options - Filter options
     * @returns Performance mark entries
     */
    getMarks(options?: { since?: number }): PerformanceMark[] {
      return getPerformanceMarks(options) as unknown as PerformanceMark[];
    },

    /**
     * Get performance measures
     * @param options - Filter options
     * @returns Performance measure entries
     */
    getMeasures(options?: { since?: number }): PerformanceMeasure[] {
      return getPerformanceMeasures(options) as unknown as PerformanceMeasure[];
    },

    // === AI Context ===

    /**
     * Enrich an error entry with AI context
     * @param error - Error entry to enrich
     * @returns Enriched error entry
     */
    enrichError(error: LogEntry): Promise<LogEntry> {
      return enrichErrorWithAiContext(error as unknown as any) as unknown as Promise<LogEntry>;
    },

    /**
     * Enable or disable AI context enrichment
     * @param enabled
     */
    setAiContext(enabled: boolean): void {
      setAiContextEnabled(enabled);
    },

    /**
     * Enable or disable state snapshot in AI context
     * @param enabled
     */
    setStateSnapshot(enabled: boolean): void {
      setAiContextStateSnapshot(enabled);
    },

    // === Reproduction Scripts ===

    /**
     * Record an enhanced action (for testing)
     * @param type - Action type
     * @param element - Target element
     * @param opts - Options
     */
    recordAction(type: string, element: Element, opts?: Record<string, unknown>): void {
      recordEnhancedAction(type as any, element, opts);
    },

    /**
     * Get the enhanced action buffer
     * @returns
     */
    getEnhancedActions(): EnhancedAction[] {
      return getEnhancedActionBuffer() as unknown as EnhancedAction[];
    },

    /**
     * Clear the enhanced action buffer
     */
    clearEnhancedActions(): void {
      clearEnhancedActionBuffer();
    },

    /**
     * Generate a Playwright reproduction script
     * @param actions - Actions to convert
     * @param opts - Generation options
     * @returns Playwright test script
     */
    generateScript(actions?: EnhancedAction[], opts?: Record<string, unknown>): string {
      return generatePlaywrightScript((actions || getEnhancedActionBuffer()) as any, opts);
    },

    /**
     * Compute multi-strategy selectors for an element
     * @param element
     * @returns
     */
    getSelectors(element: Element): SelectorStrategies {
      return computeSelectors(element) as unknown as SelectorStrategies;
    },

    /**
     * Set input value and trigger React/Vue/Svelte change events
     * Works with frameworks that track form state internally by dispatching
     * the events that frameworks listen for.
     *
     * @param selector - CSS selector for the input element
     * @param value - Value to set (string for text inputs, boolean for checkboxes)
     * @returns true if successful, false if element not found
     *
     * @example
     * // Text input
     * window.__gasoline.setInputValue('input[name="email"]', 'test@example.com')
     *
     * // Checkbox
     * window.__gasoline.setInputValue('input[type="checkbox"]', true)
     *
     * // Select dropdown
     * window.__gasoline.setInputValue('select[name="country"]', 'US')
     */
    setInputValue(selector: string, value: string | boolean): boolean {
      const element = document.querySelector(selector);
      if (!element) {
        console.error('[Gasoline] Element not found:', selector);
        return false;
      }

      try {
        // Handle different input types
        if (element instanceof HTMLInputElement) {
          if (element.type === 'checkbox' || element.type === 'radio') {
            // For checkbox/radio, use checked property
            const nativeInputCheckedSetter = Object.getOwnPropertyDescriptor(
              window.HTMLInputElement.prototype,
              'checked'
            )?.set;
            if (nativeInputCheckedSetter) {
              nativeInputCheckedSetter.call(element, Boolean(value));
            } else {
              element.checked = Boolean(value);
            }
          } else {
            // For text/email/password/etc, use value property
            const nativeInputValueSetter = Object.getOwnPropertyDescriptor(
              window.HTMLInputElement.prototype,
              'value'
            )?.set;
            if (nativeInputValueSetter) {
              nativeInputValueSetter.call(element, String(value));
            } else {
              element.value = String(value);
            }
          }
        } else if (element instanceof HTMLTextAreaElement) {
          const nativeTextAreaValueSetter = Object.getOwnPropertyDescriptor(
            window.HTMLTextAreaElement.prototype,
            'value'
          )?.set;
          if (nativeTextAreaValueSetter) {
            nativeTextAreaValueSetter.call(element, String(value));
          } else {
            element.value = String(value);
          }
        } else if (element instanceof HTMLSelectElement) {
          const nativeSelectValueSetter = Object.getOwnPropertyDescriptor(
            window.HTMLSelectElement.prototype,
            'value'
          )?.set;
          if (nativeSelectValueSetter) {
            nativeSelectValueSetter.call(element, String(value));
          } else {
            element.value = String(value);
          }
        } else {
          console.error('[Gasoline] Element is not a form input:', selector);
          return false;
        }

        // Dispatch events that React/Vue/Svelte listen for
        // These events bubble up and trigger framework change handlers

        // Input event (React 16+, Vue 3, Svelte)
        const inputEvent = new Event('input', { bubbles: true });
        element.dispatchEvent(inputEvent);

        // Change event (older React, Vue 2)
        const changeEvent = new Event('change', { bubbles: true });
        element.dispatchEvent(changeEvent);

        // Blur event (some frameworks validate on blur)
        const blurEvent = new Event('blur', { bubbles: true });
        element.dispatchEvent(blurEvent);

        return true;
      } catch (err) {
        console.error('[Gasoline] Failed to set input value:', err);
        return false;
      }
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
export function uninstallGasolineAPI(): void {
  if (typeof window !== 'undefined' && window.__gasoline) {
    delete window.__gasoline;
  }
}
