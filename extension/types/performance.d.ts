/**
 * @fileoverview Performance Types
 * Performance marks, measures, long tasks, and web vitals
 */
/**
 * Performance mark entry
 */
export interface PerformanceMark {
    readonly name: string;
    readonly startTime: number;
    readonly entryType: 'mark';
}
/**
 * Performance measure entry
 */
export interface PerformanceMeasure {
    readonly name: string;
    readonly startTime: number;
    readonly duration: number;
    readonly entryType: 'measure';
}
/**
 * Long task metrics
 */
export interface LongTaskMetrics {
    readonly count: number;
    readonly totalDuration: number;
    readonly maxDuration: number;
    readonly tasks: ReadonlyArray<{
        readonly duration: number;
        readonly startTime: number;
    }>;
}
/**
 * Web Vitals metrics
 */
export interface WebVitals {
    readonly fcp?: number;
    readonly lcp?: number;
    readonly cls?: number;
    readonly inp?: number;
}
/**
 * Performance snapshot payload
 */
export interface PerformanceSnapshot {
    readonly ts: string;
    readonly url: string;
    readonly vitals: WebVitals;
    readonly longTasks: LongTaskMetrics;
    readonly resources: {
        readonly count: number;
        readonly totalSize: number;
        readonly byType: Readonly<Record<string, {
            count: number;
            size: number;
        }>>;
        readonly slowest: ReadonlyArray<{
            readonly url: string;
            readonly duration: number;
            readonly size: number;
        }>;
    };
    readonly memory?: {
        readonly usedJSHeapSize: number;
        readonly totalJSHeapSize: number;
    };
}
//# sourceMappingURL=performance.d.ts.map