/**
 * Purpose: Anonymous telemetry beacons for error visibility. Disable with the Kaboom telemetry opt-out key.
 */
/**
 * Fire an anonymous telemetry beacon. Fire-and-forget, never throws.
 * Uses navigator.sendBeacon when available, falls back to fetch.
 */
export declare function beacon(event: string, props?: Record<string, string>): void;
//# sourceMappingURL=telemetry-beacon.d.ts.map