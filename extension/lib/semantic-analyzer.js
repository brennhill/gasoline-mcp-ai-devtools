/**
 * semantic-analyzer.js
 * Heuristics for page classification and CTA extraction.
 */

export function analyzePage(mode) {
  const result = {
    url: window.location.href,
    title: (document.title || '').substring(0, 120),
    type: 'generic'
  };

  // 1. Page Classification
  const isLoginPage = !!document.querySelector('input[type="password"]');
  const isErrorPage = !!(document.title.match(/404|not found|error|500/i));

  if (isLoginPage) {
    result.type = 'login';
  } else if (isErrorPage) {
    result.type = 'error_page';
  }

  // 2. CTA Extraction (Simple v1)
  const primary_actions = [];
  const candidates = document.querySelectorAll('button, [role="button"], [onclick], a[href]');
  
  for (const el of candidates) {
    if (primary_actions.length >= 5) break;
    
    // Visibility check (simplified for JSDOM)
    const rect = el.getBoundingClientRect ? el.getBoundingClientRect() : { width: 1, height: 1 };
    if (rect.width === 0 || rect.height === 0) continue;

    const label = (el.textContent || el.getAttribute('aria-label') || el.getAttribute('title') || '').trim();
    if (!label) continue;

    primary_actions.push({
      label: label.substring(0, 40),
      tag: el.tagName ? el.tagName.toLowerCase() : 'div',
      selector: 'mock-selector' // Actual selector generation will be in final script
    });
  }

  result.primary_actions = primary_actions;

  // 3. Mode Handling
  if (mode === 'compact') {
    result.content_preview = (document.body.innerText || '').substring(0, 300);
    result.headings = []; // Placeholder for H1 extraction
  }

  return result;
}
