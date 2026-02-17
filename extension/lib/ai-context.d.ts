/**
 * Purpose: Provides shared runtime utilities used by extension and server workflows.
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview AI-preprocessed error enrichment pipeline.
 * Parses stack traces, resolves source maps, extracts code snippets,
 * detects UI frameworks (React/Vue/Svelte), captures state snapshots,
 * and generates AI-friendly error summaries. All within a timeout guard.
 */
import type { LogEntry, AiContextData, ParsedSourceMap } from '../types/index';
/**
 * Parsed stack frame (internal representation with nullable functionName)
 */
interface InternalStackFrame {
    functionName: string | null;
    filename: string;
    lineno: number;
    colno: number;
}
/**
 * Code snippet line entry
 */
interface SnippetLine {
    line: number;
    text: string;
    isError?: boolean;
}
/**
 * Source snippet with file and line info
 */
interface InternalSourceSnippet {
    file: string;
    line: number;
    snippet: SnippetLine[];
}
/**
 * Framework detection result
 */
interface FrameworkDetection {
    framework: 'react' | 'vue' | 'svelte';
    key?: string;
}
/**
 * React component ancestry entry
 */
interface ReactComponentEntry {
    name: string;
    propKeys?: string[];
    hasState?: boolean;
    stateKeys?: string[];
}
/**
 * React fiber node (partial typing for what we access)
 */
interface ReactFiber {
    type?: {
        displayName?: string;
        name?: string;
    } | string;
    memoizedProps?: Record<string, unknown>;
    memoizedState?: Record<string, unknown> | unknown[] | null;
    return?: ReactFiber | null;
}
/**
 * Component ancestry result
 */
interface ComponentAncestryResult {
    framework: 'react';
    components: ReactComponentEntry[];
}
/**
 * Redux store interface
 */
interface ReduxStore {
    getState: () => Record<string, unknown>;
}
/**
 * State snapshot result
 */
interface StateSnapshotResult {
    source: 'redux';
    keys: Record<string, {
        type: string;
    }>;
    relevantSlice: Record<string, unknown>;
}
/**
 * AI summary generation data
 */
interface AiSummaryData {
    errorType: string;
    message: string;
    file: string | null;
    line: number | null;
    componentAncestry: ComponentAncestryResult | null;
    stateSnapshot: StateSnapshotResult | null;
}
/**
 * Enriched error entry with AI context
 */
type EnrichedErrorEntry = LogEntry & {
    _aiContext?: AiContextData;
    _enrichments?: string[];
};
/**
 * Element with framework markers
 */
interface FrameworkElement {
    __vueParentComponent?: unknown;
    __vue_app__?: unknown;
    __svelte_meta?: unknown;
    [key: string]: unknown;
}
declare global {
    interface Window {
        __REDUX_STORE__?: ReduxStore;
    }
}
export declare function parseStackFrames(stack: string | undefined): InternalStackFrame[];
/**
 * Parse an inline base64 source map data URL
 * @param dataUrl - The data: URL containing the source map
 * @returns Parsed source map or null
 */
export declare function parseSourceMap(dataUrl: string | undefined | null): ParsedSourceMap | null;
/**
 * Extract a code snippet around a given line number
 * @param sourceContent - The full source file content
 * @param line - The 1-based line number of the error
 * @returns Array of { line, text, isError? } or null
 */
export declare function extractSnippet(sourceContent: string | undefined | null, line: number): SnippetLine[] | null;
/**
 * Source map lookup for extractSourceSnippets
 */
type SourceMapLookup = Record<string, ParsedSourceMap>;
/**
 * Extract source snippets for multiple stack frames
 * @param frames - Parsed stack frames
 * @param mockSourceMaps - Map of filename to parsed source map
 * @returns Array of snippet objects
 */
export declare function extractSourceSnippets(frames: InternalStackFrame[], mockSourceMaps: SourceMapLookup): Promise<InternalSourceSnippet[]>;
/**
 * Detect which UI framework an element belongs to
 * @param element - The DOM element (or element-like object)
 * @returns { framework, key? } or null
 */
export declare function detectFramework(element: FrameworkElement | null | undefined): FrameworkDetection | null;
/**
 * Walk a React fiber tree to extract component ancestry
 * @param fiber - The React fiber node
 * @returns Array of { name, propKeys?, hasState?, stateKeys? } in root-first order
 */
export declare function getReactComponentAncestry(fiber: ReactFiber | null | undefined): ReactComponentEntry[] | null;
/**
 * Capture application state snapshot from known store patterns.
 *
 * STATE RELEVANCE MATCHING STRATEGY:
 * 1. Extract error keywords from the error message (words > 2 chars).
 * 2. Build a "relevant slice" by matching nested state keys against common error state
 *    keys ('error', 'loading', 'status', 'failed') and error message keywords.
 * 3. Caps at AI_CONTEXT_MAX_RELEVANT_SLICE entries; values truncated at MAX_VALUE_LENGTH.
 *
 * NOTE: Only supports Redux. Other state management would need additional window.__* patterns.
 */
export declare function captureStateSnapshot(errorMessage: string): StateSnapshotResult | null;
/**
 * Generate a template-based AI summary from enrichment data
 * @param data - { errorType, message, file, line, componentAncestry, stateSnapshot }
 * @returns Summary string
 */
export declare function generateAiSummary(data: AiSummaryData): string;
/**
 * Error entry for enrichment (partial typing for what we access)
 */
interface ErrorEntryForEnrichment {
    stack?: string;
    message?: string;
}
export declare function enrichErrorWithAiContext(error: ErrorEntryForEnrichment): Promise<EnrichedErrorEntry>;
/**
 * Enable or disable AI context enrichment
 * @param enabled
 */
export declare function setAiContextEnabled(enabled: boolean): void;
/**
 * Enable or disable state snapshot in AI context
 * @param enabled
 */
export declare function setAiContextStateSnapshot(enabled: boolean): void;
/**
 * Cache a parsed source map for a URL
 * @param url - The script URL
 * @param map - The parsed source map
 */
export declare function setSourceMapCache(url: string, map: ParsedSourceMap): void;
/**
 * Get a cached source map
 * @param url - The script URL
 * @returns The cached source map or null
 */
export declare function getSourceMapCache(url: string): ParsedSourceMap | null;
/**
 * Get the number of cached source maps
 * @returns
 */
export declare function getSourceMapCacheSize(): number;
/**
 * Reset all module state for testing purposes
 * Clears source map cache and restores default settings.
 * Call this in beforeEach/afterEach test hooks to prevent test pollution.
 */
export declare function resetForTesting(): void;
export {};
//# sourceMappingURL=ai-context.d.ts.map