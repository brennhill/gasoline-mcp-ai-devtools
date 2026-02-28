// interact-explore.ts — explore_page compound command handler (#338).
// Combines page metadata, interactive elements, readable text, and navigation
// links into a single extension response, reducing MCP round-trips for AI agents.
import { domPrimitiveListInteractive } from '../dom-primitives-list-interactive.js';
import { domPrimitiveNavDiscovery } from '../dom-primitives-nav-discovery.js';
import { registerCommand } from './registry.js';
// =============================================================================
// READABLE CONTENT EXTRACTION (self-contained for chrome.scripting.executeScript)
// =============================================================================
/**
 * Self-contained script that extracts readable text content from the page.
 * Mirrors the logic of getReadableScript in tools_interact_content.go but
 * as a function reference suitable for chrome.scripting.executeScript.
 */
function readableContentScript() {
    function cleanText(el) {
        if (!el)
            return '';
        const clone = el.cloneNode(true);
        const removeTags = [
            'nav', 'header', 'footer', 'aside', 'script', 'style', 'noscript', 'svg',
            '[role="navigation"]', '[role="banner"]', '[role="contentinfo"]', '[aria-hidden="true"]',
            '.ad,.ads,.advertisement,.social-share,.comments,.sidebar,.related-posts,.newsletter'
        ];
        for (const sel of removeTags) {
            const els = clone.querySelectorAll(sel);
            for (const child of Array.from(els))
                child.remove();
        }
        return (clone.innerText || clone.textContent || '').replace(/\s+/g, ' ').trim();
    }
    function findMainContent() {
        const candidates = [
            'article', 'main', '[role="main"]', '.post-content', '.entry-content',
            '.article-body', '.article-content', '.story-body', '#content', '.content'
        ];
        for (const sel of candidates) {
            const el = document.querySelector(sel);
            if (el) {
                const text = cleanText(el);
                if (text.length > 100)
                    return { el, text };
            }
        }
        return { el: document.body, text: cleanText(document.body) };
    }
    function getByline() {
        const selectors = ['.author', '[rel="author"]', '.byline', '.post-author', 'meta[name="author"]'];
        for (const sel of selectors) {
            const el = document.querySelector(sel);
            if (el) {
                const text = (el.getAttribute('content') || el.innerText || '').trim();
                if (text.length > 0 && text.length < 200)
                    return text;
            }
        }
        return '';
    }
    const found = findMainContent();
    const content = found.text;
    const excerpt = content.slice(0, 300);
    const words = content.split(/\s+/).filter(Boolean);
    return {
        title: document.title || '',
        content,
        excerpt,
        byline: getByline(),
        word_count: words.length,
        url: window.location.href
    };
}
// =============================================================================
// EXPLORE_PAGE COMMAND (#338)
// =============================================================================
registerCommand('explore_page', async (ctx) => {
    try {
        // If URL is provided, navigate first
        const targetUrl = typeof ctx.params.url === 'string' ? ctx.params.url : undefined;
        if (targetUrl) {
            // Validate URL scheme — only http/https allowed (security: prevent javascript:/data:/chrome: injection)
            if (!/^https?:\/\//i.test(targetUrl)) {
                throw new Error('Only http/https URLs are supported for explore_page navigation, got: ' + targetUrl.split(':')[0] + ':');
            }
            // Register onUpdated listener BEFORE calling tabs.update to prevent race condition
            // where the page load completes before the listener is attached (#9.3.2, #9.7.5).
            await new Promise((resolve) => {
                const timeout = setTimeout(() => {
                    chrome.tabs.onUpdated.removeListener(onUpdated);
                    resolve();
                }, 15000);
                const onUpdated = (tabId, changeInfo) => {
                    if (tabId === ctx.tabId && changeInfo.status === 'complete') {
                        chrome.tabs.onUpdated.removeListener(onUpdated);
                        clearTimeout(timeout);
                        resolve();
                    }
                };
                chrome.tabs.onUpdated.addListener(onUpdated);
                chrome.tabs.update(ctx.tabId, { url: targetUrl }).catch(() => {
                    chrome.tabs.onUpdated.removeListener(onUpdated);
                    clearTimeout(timeout);
                    resolve(); // continue with current page state
                });
            });
        }
        // 1. Get tab info (page metadata)
        const tab = await chrome.tabs.get(ctx.tabId);
        // 2. Run all data collection in parallel — capture errors for _errors array (#9.7.4)
        const [interactiveResults, readableResults, navResults] = await Promise.all([
            // Interactive elements
            chrome.scripting.executeScript({
                target: { tabId: ctx.tabId, allFrames: true },
                world: 'MAIN',
                func: domPrimitiveListInteractive,
                args: ['']
            }).catch((err) => [{ result: { success: false, error: err.message, _source: 'interactive' } }]),
            // Readable content
            chrome.scripting.executeScript({
                target: { tabId: ctx.tabId },
                world: 'ISOLATED',
                func: readableContentScript
            }).catch((err) => [{ result: { error: 'extraction_failed', _reason: err.message, _source: 'readable' } }]),
            // Navigation links (uses shared dom-primitives-nav-discovery)
            chrome.scripting.executeScript({
                target: { tabId: ctx.tabId },
                world: 'ISOLATED',
                func: domPrimitiveNavDiscovery
            }).catch((err) => [{ result: { error: 'extraction_failed', _reason: err.message, _source: 'navigation' } }])
        ]);
        // Process interactive elements (capped at 100)
        const elements = [];
        let interactiveError;
        for (const r of interactiveResults) {
            const res = r.result;
            if (res?.success === false) {
                if (!interactiveError)
                    interactiveError = res.error || res.message;
                continue;
            }
            if (res?.elements) {
                elements.push(...res.elements);
                if (elements.length >= 100)
                    break;
            }
        }
        let cappedElements = elements.slice(0, 100);
        // Apply visible_only filter if requested
        if (ctx.params.visible_only === true) {
            cappedElements = cappedElements.filter((el) => {
                const elem = el;
                return elem.visible !== false;
            });
        }
        // Apply limit if specified
        const limit = typeof ctx.params.limit === 'number' && ctx.params.limit > 0
            ? ctx.params.limit
            : cappedElements.length;
        const finalElements = cappedElements.slice(0, limit);
        // Process readable content
        const readableFirst = readableResults?.[0]?.result;
        const readable = readableFirst && typeof readableFirst === 'object'
            ? readableFirst
            : null;
        // Process navigation links
        const navFirst = navResults?.[0]?.result;
        const navigation = navFirst && typeof navFirst === 'object'
            ? navFirst
            : null;
        // Build composite payload
        const payload = {
            // Page metadata
            url: tab.url || '',
            title: tab.title || '',
            tab_status: tab.status || '',
            favicon: tab.favIconUrl || '',
            viewport: {
                width: tab.width,
                height: tab.height
            },
            // Interactive elements
            interactive_elements: finalElements,
            interactive_count: finalElements.length,
            // Readable text
            readable: readable || { error: 'extraction_failed' },
            // Navigation links
            navigation: navigation || { error: 'extraction_failed' }
        };
        // Build unified _errors array for partial failures (UX Review R6)
        const errors = [];
        if (interactiveError && finalElements.length === 0) {
            payload.interactive_error = interactiveError;
            errors.push({ component: 'interactive', error: interactiveError });
        }
        if (readable && typeof readable === 'object' && 'error' in readable) {
            const reason = readable._reason;
            errors.push({ component: 'readable', error: String(reason || readable.error) });
        }
        if (navigation && typeof navigation === 'object' && 'error' in navigation) {
            const reason = navigation._reason;
            errors.push({ component: 'navigation', error: String(reason || navigation.error) });
        }
        if (errors.length > 0) {
            payload._errors = errors;
        }
        if (ctx.query.correlation_id) {
            ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, 'complete', payload);
        }
        else {
            ctx.sendResult(payload);
        }
    }
    catch (err) {
        const message = err.message || 'Explore page failed';
        if (ctx.query.correlation_id) {
            ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, 'error', null, message);
        }
        else {
            ctx.sendResult({
                error: 'explore_page_failed',
                message
            });
        }
    }
});
//# sourceMappingURL=interact-explore.js.map