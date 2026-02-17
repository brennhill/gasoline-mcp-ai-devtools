/**
 * Purpose: Owns sourcemap.ts runtime behavior and integration logic.
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview Source Map Types
 * Source map parsing and original location resolution
 */
/**
 * Parsed source map
 */
export interface ParsedSourceMap {
    readonly sources: readonly string[];
    readonly names: readonly string[];
    readonly sourceRoot: string;
    readonly mappings: readonly (readonly (readonly number[])[])[];
    readonly sourcesContent: readonly string[];
}
/**
 * Original location from source map
 */
export interface OriginalLocation {
    readonly source: string;
    readonly line: number;
    readonly column: number;
    readonly name: string | null;
}
//# sourceMappingURL=sourcemap.d.ts.map