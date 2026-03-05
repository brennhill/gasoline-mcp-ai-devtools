/**
 * Purpose: In-browser terminal widget orchestrator — public API, write guard, mount/unmount.
 * Why: Provides a Lovable-like experience — chat with any CLI (claude, codex, aider) from
 * a browser overlay while seeing code edits reflected via hot reload on the tracked page.
 * Docs: docs/features/feature/terminal/index.md
 */
import { type TerminalConfig } from './terminal-widget-types.js';
import { saveTerminalConfig } from './terminal-widget-session.js';
export declare function hideTerminal(): void;
/** Kill the PTY session on the daemon and tear down the widget completely. */
export declare function exitTerminalSession(): Promise<void>;
export declare function showTerminal(): void;
export declare function isTerminalVisible(): boolean;
export declare function toggleTerminal(): Promise<void>;
/** Restore terminal on page load if it was previously open/minimized. */
export declare function restoreTerminalIfNeeded(): Promise<void>;
/** Write text to the terminal PTY stdin via the iframe postMessage bridge, then press Enter to submit. */
export declare function writeToTerminal(text: string): void;
export { saveTerminalConfig };
export type { TerminalConfig };
/** Reset all shared state — test-only. Needed because split sub-modules are cached by Node ESM. */
export declare function _resetForTesting(): void;
//# sourceMappingURL=terminal-widget.d.ts.map