/**
 * @fileoverview AI-preprocessed error enrichment pipeline.
 * Parses stack traces, resolves source maps, extracts code snippets,
 * detects UI frameworks (React/Vue/Svelte), captures state snapshots,
 * and generates AI-friendly error summaries. All within a timeout guard.
 *
 * This file re-exports from ai-context-parsing.ts and ai-context-enrichment.ts
 * so existing importers are unaffected.
 */
export { parseStackFrames, parseSourceMap, extractSnippet, extractSourceSnippets, setSourceMapCache, getSourceMapCache, getSourceMapCacheSize, resetParsingForTesting } from './ai-context-parsing.js';
export { detectFramework, getReactComponentAncestry, captureStateSnapshot, generateAiSummary, enrichErrorWithAiContext, setAiContextEnabled, setAiContextStateSnapshot, resetEnrichmentForTesting } from './ai-context-enrichment.js';
/**
 * Reset all module state for testing purposes
 * Clears source map cache and restores default settings.
 * Call this in beforeEach/afterEach test hooks to prevent test pollution.
 */
export declare function resetForTesting(): void;
//# sourceMappingURL=ai-context.d.ts.map