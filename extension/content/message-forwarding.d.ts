/**
 * Purpose: Handles content-script message relay between background and inject contexts.
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/query-dom/index.md
 */
/**
 * @fileoverview Message Forwarding Module
 * Forwards messages between page context and background script
 */
import type { BackgroundMessageFromContent } from './types';
export declare const MESSAGE_MAP: Record<string, string>;
/**
 * Safely send a message to the background script
 * Handles extension context invalidation gracefully
 */
export declare function safeSendMessage(msg: BackgroundMessageFromContent): void;
/**
 * Check if the extension context is still valid
 */
export declare function isContextValid(): boolean;
//# sourceMappingURL=message-forwarding.d.ts.map