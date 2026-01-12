/**
 * @fileoverview Network Types
 * Network waterfall, request tracking, and body capture
 */

/**
 * Network waterfall entry phases
 */
export interface WaterfallPhases {
  readonly dns: number;
  readonly connect: number;
  readonly tls: number;
  readonly ttfb: number;
  readonly download: number;
}

/**
 * Parsed network waterfall entry
 */
export interface WaterfallEntry {
  readonly url: string;
  readonly initiatorType: string;
  readonly startTime: number;
  readonly duration: number;
  readonly phases: WaterfallPhases;
  readonly transferSize: number;
  readonly encodedBodySize: number;
  readonly decodedBodySize: number;
  readonly cached?: boolean;
}

/**
 * Pending network request tracking
 */
export interface PendingRequest {
  readonly id: string;
  readonly url: string;
  readonly method: string;
  readonly startTime: number;
}

/**
 * Network body capture payload
 */
export interface NetworkBodyPayload {
  readonly url: string;
  readonly method: string;
  readonly status: number;
  readonly contentType: string;
  readonly requestBody?: string;
  readonly responseBody?: string;
  readonly duration: number;
}
