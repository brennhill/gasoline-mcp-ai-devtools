/**
 * Purpose: Fetches and caches source maps, parses stack frames with VLQ decoding, and resolves stack traces for better error messages.
 * Docs: docs/features/feature/observe/index.md
 */
import type { ParsedSourceMap } from '../types/index.js';
/** Parsed stack frame */
interface ParsedStackFrame {
    functionName: string;
    fileName: string;
    lineNumber: number;
    columnNumber: number;
    raw: string;
    originalFileName?: string;
    originalLineNumber?: number;
    originalColumnNumber?: number;
    originalFunctionName?: string;
    resolved?: boolean;
}
/** Original location from source map */
interface OriginalLocation {
    source: string;
    line: number;
    column: number;
    name: string | null;
}
/**
 * Decode a VLQ-encoded string into an array of integers
 */
export declare function decodeVLQ(str: string): number[];
/**
 * Parse a source map's mappings string into a structured format
 */
export declare function parseMappings(mappingsStr: string): number[][][];
/**
 * Parse a stack trace line into components
 */
export declare function parseStackFrame(line: string): ParsedStackFrame | null;
/**
 * Extract sourceMappingURL from script content
 */
export declare function extractSourceMapUrl(content: string): string | null;
/**
 * Parse source map data into a usable format
 */
export declare function parseSourceMapData(sourceMap: {
    mappings?: string;
    sources?: string[];
    names?: string[];
    sourceRoot?: string;
    sourcesContent?: string[];
}): ParsedSourceMap;
/**
 * Find original location from source map
 */
export declare function findOriginalLocation(sourceMap: ParsedSourceMap, line: number, column: number): OriginalLocation | null;
type DebugLogFn = (category: string, message: string, data?: unknown) => void;
export declare function fetchSourceMap(scriptUrl: string, debugLogFn?: DebugLogFn): Promise<ParsedSourceMap | null>;
/**
 * Resolve a single stack frame to original location
 */
export declare function resolveStackFrame(frame: ParsedStackFrame, debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<ParsedStackFrame>;
/**
 * Resolve an entire stack trace
 */
export declare function resolveStackTrace(stack: string, debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<string>;
export {};
//# sourceMappingURL=source-map-resolver.d.ts.map