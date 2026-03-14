/**
 * Purpose: Extracts comprehensive page metadata (viewport, headings, forms, links) for page-level context.
 * Why: Separates page-level metadata extraction from per-element DOM querying so each can evolve independently.
 * Docs: docs/features/feature/query-dom/index.md
 */
import { DOM_QUERY_MAX_TEXT } from './constants.js';
/**
 * Get comprehensive page info
 */
export async function getPageInfo() {
    const headings = [];
    const headingEls = document.querySelectorAll('h1,h2,h3,h4,h5,h6');
    for (const h of headingEls) {
        headings.push((h.textContent || '').slice(0, DOM_QUERY_MAX_TEXT));
    }
    const forms = [];
    const formEls = document.querySelectorAll('form');
    for (const form of formEls) {
        const fields = [];
        const inputs = form.querySelectorAll('input,select,textarea');
        for (const input of inputs) {
            const inputEl = input;
            if (inputEl.name)
                fields.push(inputEl.name);
        }
        forms.push({
            id: form.id || undefined,
            action: form.action || undefined,
            fields
        });
    }
    return {
        url: window.location.href,
        title: document.title,
        viewport: { width: window.innerWidth, height: window.innerHeight },
        scroll: { x: window.scrollX, y: window.scrollY },
        documentHeight: document.documentElement.scrollHeight,
        headings,
        links: document.querySelectorAll('a').length,
        images: document.querySelectorAll('img').length,
        interactiveElements: document.querySelectorAll('button,input,select,textarea,a[href]').length,
        forms
    };
}
//# sourceMappingURL=page-info.js.map