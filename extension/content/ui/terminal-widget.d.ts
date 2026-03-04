/**
 * Purpose: In-browser terminal widget that embeds a PTY-backed terminal via iframe.
 * Why: Provides a Lovable-like experience — chat with any CLI (claude, codex, aider) from
 * a browser overlay while seeing code edits reflected via hot reload on the tracked page.
 * Docs: docs/features/feature/terminal/index.md
 */
interface TerminalConfig {
    cmd?: string;
    args?: string[];
    dir?: string;
    serverUrl?: string;
}
declare function saveTerminalConfig(config: TerminalConfig): void;
export declare function hideTerminal(): void;
export declare function showTerminal(): void;
export declare function isTerminalVisible(): boolean;
export declare function toggleTerminal(): Promise<void>;
export declare function unmountTerminal(): void;
export { saveTerminalConfig };
export type { TerminalConfig };
//# sourceMappingURL=terminal-widget.d.ts.map