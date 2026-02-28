// interact-content.ts — Command handlers for content extraction query types (#257).
// Handles: get_readable, get_markdown, page_summary.
// Routes through chrome.tabs.sendMessage to the content script (ISOLATED world, CSP-safe).
// Falls back to chrome.scripting.executeScript when content script is not loaded (#364).
import { registerCommand } from './registry.js';
function isContentScriptUnreachableError(err) {
    const message = err?.message || '';
    return message.includes('Receiving end does not exist') || message.includes('Could not establish connection');
}
/**
 * Inline readable extractor for chrome.scripting.executeScript fallback.
 * Self-contained — no external references. Mirrors src/content/extractors/readable.ts logic.
 */
function readableFallbackScript() {
    const MAIN_SELECTORS = [
        'main', 'article', '[role="main"]', '#main', '.main',
        '.post-content', '.entry-content', '.article-body', '.article-content',
        '.story-body', '.article', '.post', '#content', '.content', '.results'
    ];
    const REMOVE_SELECTORS = [
        'nav', 'header', 'footer', 'aside', 'script', 'style', 'noscript', 'svg',
        '[role="navigation"]', '[role="banner"]', '[role="contentinfo"]', '[aria-hidden="true"]',
        '.ad', '.ads', '.advertisement', '.social-share', '.comments', '.sidebar',
        '.related-posts', '.newsletter'
    ];
    let mainEl = document.body || document.documentElement;
    for (const sel of MAIN_SELECTORS) {
        const el = document.querySelector(sel);
        if (!el)
            continue;
        const text = (el.innerText || el.textContent || '').trim();
        if (text.length > 100) {
            mainEl = el;
            break;
        }
    }
    const clone = mainEl.cloneNode(true);
    for (const sel of REMOVE_SELECTORS) {
        for (const child of Array.from(clone.querySelectorAll(sel)))
            child.remove();
    }
    const content = (clone.innerText || clone.textContent || '').replace(/\s+/g, ' ').trim();
    let byline = '';
    for (const sel of ['.author', '[rel="author"]', '.byline', '.post-author', 'meta[name="author"]']) {
        const el = document.querySelector(sel);
        if (el) {
            const text = (el.getAttribute('content') || el.innerText || '').trim();
            if (text.length > 0 && text.length < 200) {
                byline = text;
                break;
            }
        }
    }
    return {
        title: document.title || '',
        content,
        excerpt: content.slice(0, 300),
        byline,
        word_count: content.split(/\s+/).filter(Boolean).length,
        url: window.location.href,
        fallback: true
    };
}
/**
 * Inline markdown extractor for chrome.scripting.executeScript fallback.
 * Self-contained — simplified text-based extraction (no HTML-to-markdown regex).
 * Returns clean text content rather than risk broken regex conversion.
 */
function markdownFallbackScript() {
    const MAX_OUTPUT = 200000;
    const MAIN_SELECTORS = [
        'main', 'article', '[role="main"]', '#main', '.main',
        '.post-content', '.entry-content', '.article-body', '.article-content'
    ];
    const REMOVE_SELECTORS = [
        'nav', 'header', 'footer', 'aside', 'script', 'style', 'noscript', 'svg',
        '[role="navigation"]', '[role="banner"]', '[role="contentinfo"]', '[aria-hidden="true"]'
    ];
    let mainEl = document.body || document.documentElement;
    for (const sel of MAIN_SELECTORS) {
        const el = document.querySelector(sel);
        if (!el)
            continue;
        const text = (el.innerText || el.textContent || '').trim();
        if (text.length > 100) {
            mainEl = el;
            break;
        }
    }
    const clone = mainEl.cloneNode(true);
    for (const sel of REMOVE_SELECTORS) {
        for (const child of Array.from(clone.querySelectorAll(sel)))
            child.remove();
    }
    // Use innerText for natural text extraction (preserves block structure)
    let markdown = (clone.innerText || clone.textContent || '').trim();
    if (markdown.length > MAX_OUTPUT) {
        markdown = markdown.slice(0, MAX_OUTPUT);
    }
    return {
        title: document.title || '',
        markdown,
        url: window.location.href,
        word_count: markdown.split(/\s+/).filter(Boolean).length,
        fallback: true
    };
}
/**
 * Inline page summary extractor for chrome.scripting.executeScript fallback.
 * Self-contained — mirrors src/content/extractors/page-summary.ts output shape.
 */
function pageSummaryFallbackScript() {
    // Headings
    const headings = [];
    for (const heading of Array.from(document.querySelectorAll('h1, h2, h3'))) {
        if (headings.length >= 30)
            break;
        const text = (heading.innerText || heading.textContent || '').replace(/\s+/g, ' ').trim().slice(0, 200);
        if (text)
            headings.push(heading.tagName.toLowerCase() + ': ' + text);
    }
    // Navigation links
    const navCandidates = document.querySelectorAll('nav a[href], header a[href], [role="navigation"] a[href]');
    const navLinks = [];
    const seenNav = {};
    for (const link of Array.from(navCandidates)) {
        if (navLinks.length >= 25)
            break;
        const linkText = (link.innerText || link.textContent || '').replace(/\s+/g, ' ').trim().slice(0, 80);
        let href = link.getAttribute('href') || '';
        try {
            href = new URL(href, window.location.href).href;
        }
        catch { /* keep as-is */ }
        if (!href)
            continue;
        const key = linkText + '|' + href;
        if (seenNav[key])
            continue;
        seenNav[key] = true;
        navLinks.push({ text: linkText, href });
    }
    // Forms
    const forms = [];
    for (const form of Array.from(document.querySelectorAll('form'))) {
        if (forms.length >= 10)
            break;
        const fields = [];
        const seenFields = {};
        for (const field of Array.from(form.querySelectorAll('input, select, textarea'))) {
            if (fields.length >= 25)
                break;
            const name = field.getAttribute('name') || field.getAttribute('id') || field.getAttribute('aria-label') || field.getAttribute('type') || field.tagName.toLowerCase();
            const cleaned = (name || '').replace(/\s+/g, ' ').trim().slice(0, 60);
            if (!cleaned || seenFields[cleaned])
                continue;
            seenFields[cleaned] = true;
            fields.push(cleaned);
        }
        let action = form.getAttribute('action') || window.location.href;
        try {
            action = new URL(action, window.location.href).href;
        }
        catch { /* keep as-is */ }
        forms.push({ action, method: (form.getAttribute('method') || 'GET').toUpperCase(), fields });
    }
    // Main content preview
    const MAIN_SELECTORS = ['main', 'article', '[role="main"]', '#main', '.main', '.post-content', '.entry-content'];
    let mainEl = document.body || document.documentElement;
    for (const sel of MAIN_SELECTORS) {
        const el = document.querySelector(sel);
        if (!el)
            continue;
        const text = (el.innerText || el.textContent || '').trim();
        if (text.length > 120) {
            mainEl = el;
            break;
        }
    }
    const mainText = (mainEl.innerText || mainEl.textContent || '').replace(/\s+/g, ' ').trim().slice(0, 20000);
    const preview = mainText.slice(0, 500);
    const wordCount = mainText ? mainText.split(/\s+/).filter(Boolean).length : 0;
    // Interactive count (simplified — skip visibility check for fallback perf)
    const interactiveCount = document.querySelectorAll('a[href],button,input:not([type="hidden"]),select,textarea,[role="button"],[role="link"]').length;
    // Classification
    const linkCount = document.querySelectorAll('a[href]').length;
    const paragraphCount = document.querySelectorAll('p').length;
    const hasSearchInput = !!document.querySelector('input[type="search"], input[name*="search" i], input[placeholder*="search" i]');
    const likelySearchURL = /[?&](q|query|search)=/i.test(window.location.search);
    const hasArticle = document.querySelectorAll('article').length > 0;
    const hasTable = document.querySelectorAll('table').length > 0;
    let totalFormFields = 0;
    for (const f of forms)
        totalFormFields += f.fields.length;
    let type = 'generic';
    if (hasSearchInput && (likelySearchURL || linkCount > 10))
        type = 'search_results';
    else if (forms.length > 0 && totalFormFields >= 3 && paragraphCount < 8)
        type = 'form';
    else if (hasArticle || (paragraphCount >= 8 && linkCount < paragraphCount * 2))
        type = 'article';
    else if (hasTable || (interactiveCount > 25 && headings.length >= 2))
        type = 'dashboard';
    else if (linkCount > 30 && paragraphCount < 10)
        type = 'link_list';
    else if (preview.length < 80 && interactiveCount > 10)
        type = 'app';
    return {
        url: window.location.href,
        title: document.title || '',
        type,
        headings,
        nav_links: navLinks,
        forms,
        interactive_element_count: interactiveCount,
        main_content_preview: preview,
        word_count: wordCount,
        fallback: true
    };
}
const FALLBACK_SCRIPTS = {
    GASOLINE_GET_READABLE: readableFallbackScript,
    GASOLINE_GET_MARKDOWN: markdownFallbackScript,
    GASOLINE_PAGE_SUMMARY: pageSummaryFallbackScript
};
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
            // Fallback: inject extraction script directly when content script is not loaded
            if (isContentScriptUnreachableError(err)) {
                const fallbackFn = FALLBACK_SCRIPTS[messageType];
                if (fallbackFn) {
                    try {
                        const results = await chrome.scripting.executeScript({
                            target: { tabId: ctx.tabId },
                            world: 'ISOLATED',
                            func: fallbackFn
                        });
                        const firstResult = results?.[0]?.result;
                        if (firstResult) {
                            ctx.sendResult(firstResult);
                            return;
                        }
                    }
                    catch (fallbackErr) {
                        // Fallback also failed — return error with context
                        ctx.sendResult({
                            error: errorCode,
                            message: `Content script not loaded and fallback injection failed: ${fallbackErr.message || 'unknown error'}. Refresh the page first: interact({what: "refresh"}), then retry.`
                        });
                        return;
                    }
                }
                ctx.sendResult({
                    error: errorCode,
                    message: 'Content script not loaded on this page. Refresh the page first: interact({what: "refresh"}), then retry.'
                });
                return;
            }
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