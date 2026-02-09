/**
 * @fileoverview Link Health Checker
 * Extracts all links from the current page and checks their health.
 * Categorizes issues as: ok (2xx), redirect (3xx), requires_auth (401/403),
 * broken (4xx/5xx), or timeout.
 */
/**
 * Check all links on the current page for health issues.
 * Extracts links, deduplicates, and checks max 20 concurrently.
 */
export async function checkLinkHealth(params) {
    const timeout_ms = params.timeout_ms || 15000;
    const max_workers = params.max_workers || 20;
    // Extract all links from the page
    const linkElements = document.querySelectorAll('a[href]');
    const urls = new Set();
    for (const elem of linkElements) {
        const href = elem.href;
        if (href && !isIgnoredLink(href)) {
            urls.add(href);
        }
    }
    const uniqueLinks = Array.from(urls);
    // Check links in parallel batches
    const results = [];
    const chunks = chunkArray(uniqueLinks, max_workers);
    for (const chunk of chunks) {
        const batchResults = await Promise.allSettled(chunk.map((url) => checkLink(url, timeout_ms)));
        for (const result of batchResults) {
            if (result.status === 'fulfilled' && result.value) {
                results.push(result.value);
            }
        }
    }
    // Aggregate results into summary
    const summary = {
        totalLinks: results.length,
        ok: 0,
        redirect: 0,
        requiresAuth: 0,
        broken: 0,
        timeout: 0,
    };
    for (const result of results) {
        if (result.code === 'ok')
            summary.ok++;
        else if (result.code === 'redirect')
            summary.redirect++;
        else if (result.code === 'requires_auth')
            summary.requiresAuth++;
        else if (result.code === 'broken')
            summary.broken++;
        else if (result.code === 'timeout')
            summary.timeout++;
    }
    return { summary, results };
}
/**
 * Check a single link for health issues.
 */
async function checkLink(url, timeout_ms) {
    const startTime = performance.now();
    const isExternal = new URL(url).origin !== window.location.origin;
    try {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), timeout_ms);
        try {
            const response = await fetch(url, {
                method: 'HEAD',
                signal: controller.signal,
                redirect: 'follow',
            });
            clearTimeout(timeoutId);
            const timeMs = Math.round(performance.now() - startTime);
            // Categorize by status
            let code;
            if (response.status >= 200 && response.status < 300) {
                code = 'ok';
            }
            else if (response.status >= 300 && response.status < 400) {
                code = 'redirect';
            }
            else if (response.status === 401 || response.status === 403) {
                code = 'requires_auth';
            }
            else if (response.status >= 400) {
                code = 'broken';
            }
            else {
                code = 'broken';
            }
            return {
                url,
                status: response.status,
                code,
                timeMs,
                isExternal,
                redirectTo: response.redirected ? response.url : undefined,
            };
        }
        finally {
            clearTimeout(timeoutId);
        }
    }
    catch (error) {
        const timeMs = Math.round(performance.now() - startTime);
        const isTimeout = error.name === 'AbortError';
        return {
            url,
            status: null,
            code: isTimeout ? 'timeout' : 'broken',
            timeMs,
            isExternal,
            error: isTimeout ? 'timeout' : error.message,
        };
    }
}
/**
 * Determine if a link should be skipped (javascript:, mailto:, #anchor).
 */
function isIgnoredLink(href) {
    if (href.startsWith('javascript:'))
        return true;
    if (href.startsWith('mailto:'))
        return true;
    if (href.startsWith('tel:'))
        return true;
    if (href.startsWith('#'))
        return true;
    if (href === '')
        return true;
    return false;
}
/**
 * Split array into chunks of specified size.
 */
function chunkArray(arr, chunkSize) {
    const chunks = [];
    for (let i = 0; i < arr.length; i += chunkSize) {
        chunks.push(arr.slice(i, i + chunkSize));
    }
    return chunks;
}
//# sourceMappingURL=link-health.js.map