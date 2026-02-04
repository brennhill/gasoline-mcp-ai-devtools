/**
 * @fileoverview Performance snapshot capture.
 * Observes web vitals (FCP, LCP, CLS, INP), long tasks, and resource timing
 * to build comprehensive performance snapshots.
 */
interface ResourceByType {
    count: number;
    size: number;
}
interface SlowRequest {
    url: string;
    duration: number;
    size: number;
}
interface ResourceTimingSummary {
    requestCount: number;
    transferSize: number;
    decodedSize: number;
    byType: Record<string, ResourceByType>;
    slowestRequests: SlowRequest[];
}
interface LongTaskMetrics {
    count: number;
    totalBlockingTime: number;
    longest: number;
}
interface NetworkTiming {
    domContentLoaded: number;
    load: number;
    firstContentfulPaint: number | null;
    largestContentfulPaint: number | null;
    interactionToNextPaint: number | null;
    timeToFirstByte: number;
    domInteractive: number;
}
interface PerformanceSnapshotData {
    url: string;
    timestamp: string;
    timing: NetworkTiming;
    network: ResourceTimingSummary;
    longTasks: LongTaskMetrics;
    cumulativeLayoutShift: number;
}
/**
 * Map resource initiator types to standard categories
 */
export declare function mapInitiatorType(type: string): string;
/**
 * Aggregate resource timing entries into a network summary
 */
export declare function aggregateResourceTiming(): ResourceTimingSummary;
/**
 * Capture a performance snapshot with navigation timing and network summary
 */
export declare function capturePerformanceSnapshot(): PerformanceSnapshotData | null;
/**
 * Install performance observers for long tasks, paint, LCP, and CLS
 */
export declare function installPerfObservers(): void;
/**
 * Disconnect all performance observers
 */
export declare function uninstallPerfObservers(): void;
/**
 * Get accumulated long task metrics
 */
export declare function getLongTaskMetrics(): LongTaskMetrics;
/**
 * Get First Contentful Paint value
 */
export declare function getFCP(): number | null;
/**
 * Get Largest Contentful Paint value
 */
export declare function getLCP(): number | null;
/**
 * Get Cumulative Layout Shift value
 */
export declare function getCLS(): number;
/**
 * Get Interaction to Next Paint value
 */
export declare function getINP(): number | null;
/**
 * Send performance snapshot via postMessage to content script
 */
export declare function sendPerformanceSnapshot(): void;
/**
 * Check if performance snapshot capture is enabled
 */
export declare function isPerformanceSnapshotEnabled(): boolean;
/**
 * Enable or disable performance snapshot capture
 */
export declare function setPerformanceSnapshotEnabled(enabled: boolean): void;
export {};
//# sourceMappingURL=perf-snapshot.d.ts.map