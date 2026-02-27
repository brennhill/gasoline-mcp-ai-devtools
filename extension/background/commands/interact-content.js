// interact-content.ts — Command handlers for content extraction query types (#257).
// Handles: get_readable, get_markdown, page_summary.
// Routes through chrome.tabs.sendMessage to the content script (ISOLATED world, CSP-safe).
// No chrome.scripting.executeScript or eval — extraction logic lives in the content script.
import { registerCommand } from './registry.js';
/**
 * Factory for content extraction command handlers.
 * All three extractors share identical structure — they differ only in message type and error code.
 * The lifecycle's sendResult handles sync/async routing via correlation_id internally.
 */
function contentExtractorCommand(messageType, errorCode) {
    return async (ctx) => {
        try {
            const result = await chrome.tabs.sendMessage(ctx.tabId, {
                type: messageType,
                params: ctx.query.params
            });
            ctx.sendResult(result);
        }
        catch (err) {
            ctx.sendResult({
                error: errorCode,
                message: err.message || `${errorCode}`
            });
        }
    };
}
registerCommand('get_readable', contentExtractorCommand('GASOLINE_GET_READABLE', 'get_readable_failed'));
registerCommand('get_markdown', contentExtractorCommand('GASOLINE_GET_MARKDOWN', 'get_markdown_failed'));
registerCommand('page_summary', contentExtractorCommand('GASOLINE_PAGE_SUMMARY', 'page_summary_failed'));
//# sourceMappingURL=interact-content.js.map