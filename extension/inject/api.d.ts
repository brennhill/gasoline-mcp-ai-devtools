/**
 * @fileoverview Gasoline API - Exposes window.__gasoline interface for developers
 * to interact with Gasoline capture capabilities.
 */
import type { LogEntry, ActionEntry, EnhancedAction, SelectorStrategies, WaterfallEntry, PerformanceMark, PerformanceMeasure } from '../types/index';
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
    getNetworkWaterfall(options?: {
        since?: number;
        initiatorTypes?: string[];
    }): WaterfallEntry[];
    setPerformanceMarks(enabled: boolean): void;
    getMarks(options?: {
        since?: number;
    }): PerformanceMark[];
    getMeasures(options?: {
        since?: number;
    }): PerformanceMeasure[];
    enrichError(error: LogEntry): Promise<LogEntry>;
    setAiContext(enabled: boolean): void;
    setStateSnapshot(enabled: boolean): void;
    recordAction(type: string, element: Element, opts?: Record<string, unknown>): void;
    getEnhancedActions(): EnhancedAction[];
    clearEnhancedActions(): void;
    generateScript(opts?: Record<string, unknown>): string;
    getSelectors(element: Element): SelectorStrategies;
    setInputValue(selector: string, value: string | boolean): boolean;
    version: string;
}
declare global {
    interface Window {
        __gasoline?: GasolineAPI;
    }
}
/**
 * Install the window.__gasoline API for developers to interact with Gasoline
 */
export declare function installGasolineAPI(): void;
/**
 * Uninstall the window.__gasoline API
 */
export declare function uninstallGasolineAPI(): void;
//# sourceMappingURL=api.d.ts.map