/**
 * Purpose: Replaces the page favicon with the Kaboom flame icon when tab tracking is enabled and adds flickering animation when AI Pilot is active.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
/**
 * @fileoverview Favicon Replacer - Visual indicator for tracked tabs
 * Replaces the page's favicon with the Kaboom flame icon when tab tracking is enabled.
 * Adds flickering animation when AI Pilot is active.
 */
import type { TrackingState } from '../types/index.js';
/**
 * Initialize favicon replacement.
 * Requests initial tracking state from background and updates favicon accordingly.
 * Ongoing tracking_state_changed messages are handled by the central runtime-message-listener.
 */
export declare function initFaviconReplacer(): void;
/**
 * Update favicon based on tracking state.
 * - Not tracked: Shows original favicon
 * - Tracked (AI Pilot off): Shows static glowing flame
 * - Tracked (AI Pilot on): Shows flickering flame
 */
export declare function updateFavicon(state: TrackingState): void;
/**
 * Restore the original page favicon (exported for use by central message handler).
 */
export declare function restoreFavicon(): void;
//# sourceMappingURL=favicon-replacer.d.ts.map