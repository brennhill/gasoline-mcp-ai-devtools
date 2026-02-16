// tools_page_summary.go — Page summary script and helpers.
// Contains the pageSummaryScript IIFE and mode-specific builders.
package main

import "fmt"

// pageSummaryScript is a self-contained IIFE that analyzes the current page.
// Accepts a mode parameter: 'compact' (navigate-bundled) or 'full' (standalone).
const pageSummaryScript = `(function (mode) {
  function cleanText(value, maxLen) {
    var text = (value || '').replace(/\s+/g, ' ').trim();
    if (maxLen > 0 && text.length > maxLen) {
      return text.slice(0, maxLen);
    }
    return text;
  }

  function absoluteHref(value) {
    try {
      return new URL(value || '', window.location.href).href;
    } catch (_err) {
      return value || '';
    }
  }

  function visibleInteractiveCount() {
    var nodes = document.querySelectorAll(
      'a[href],button,input:not([type="hidden"]),select,textarea,[role="button"],[role="link"],[tabindex]'
    );
    var count = 0;
    for (var i = 0; i < nodes.length; i++) {
      var node = nodes[i];
      if (node.disabled) continue;
      var style = window.getComputedStyle(node);
      if (style.display === 'none' || style.visibility === 'hidden') continue;
      var rect = node.getBoundingClientRect();
      if (rect.width <= 0 || rect.height <= 0) continue;
      count += 1;
    }
    return count;
  }

  function findMainNode() {
    var candidates = [
      'main',
      'article',
      '[role="main"]',
      '#main',
      '.main',
      '.content',
      '#content',
      '.article',
      '.post',
      '.results'
    ];
    for (var i = 0; i < candidates.length; i++) {
      var node = document.querySelector(candidates[i]);
      if (!node) continue;
      var text = cleanText(node.innerText || node.textContent || '', 0);
      if (text.length > 120) {
        return node;
      }
    }
    return document.body || document.documentElement;
  }

  function classifyPage(forms, interactiveCount, linkCount, paragraphCount, headingCount, previewText) {
    // Phase 1 Improvements: Login and Error detection
    if (document.querySelector('input[type="password"]')) {
      return 'login';
    }
    if (document.title.match(/404|not found|error|500/i) && (previewText || '').length < 200) {
      return 'error_page';
    }

    var hasSearchInput = !!document.querySelector(
      'input[type="search"], input[name*="search" i], input[placeholder*="search" i]'
    );
    var likelySearchURL = /[?&](q|query|search)=/i.test(window.location.search);
    var hasArticle = document.querySelectorAll('article').length > 0;
    var hasTable = document.querySelectorAll('table').length > 0;
    var totalFormFields = 0;
    for (var i = 0; i < forms.length; i++) {
      totalFormFields += (forms[i].fields || []).length;
    }

    if (hasSearchInput && (likelySearchURL || linkCount > 10)) {
      return 'search_results';
    }
    if (forms.length > 0 && totalFormFields >= 3 && paragraphCount < 8) {
      return 'form';
    }
    if (hasArticle || (paragraphCount >= 8 && linkCount < paragraphCount * 2)) {
      return 'article';
    }
    if (hasTable || (interactiveCount > 25 && headingCount >= 2)) {
      return 'dashboard';
    }
    if (linkCount > 30 && paragraphCount < 10) {
      return 'link_list';
    }
    if ((previewText || '').length < 80 && interactiveCount > 10) {
      return 'app';
    }
    return 'generic';
  }

  function getSelector(el) {
    if (el.id) return '#' + el.id;
    var name = el.getAttribute('name');
    if (name) return "[name='" + name + "']";
    var aria = el.getAttribute('aria-label');
    if (aria) return "aria-label=" + aria;
    var text = (el.textContent || '').trim().substring(0, 20);
    if (text) return "text=" + text;
    return el.tagName.toLowerCase();
  }

  function extractPrimaryActions(candidates) {
    var actions = [];
    var seen = {};
    for (var i = 0; i < candidates.length && actions.length < 5; i++) {
      var el = candidates[i];
      var label = (el.textContent || el.getAttribute('aria-label') || el.getAttribute('title') || '').trim();
      if (!label || seen[label]) continue;
      
      var rect = el.getBoundingClientRect();
      if (rect.width <= 0 || rect.height <= 0) continue;

      seen[label] = true;
      actions.push({
        label: cleanText(label, 40),
        tag: el.tagName.toLowerCase(),
        selector: getSelector(el)
      });
    }
    return actions;
  }

  var headingNodes = document.querySelectorAll('h1, h2, h3');
  var headings = [];
  var headingLimit = mode === 'compact' ? 3 : 30;
  for (var i = 0; i < headingNodes.length && headings.length < headingLimit; i++) {
    var heading = headingNodes[i];
    if (mode === 'compact' && heading.tagName !== 'H1') continue;
    var text = cleanText(heading.innerText || heading.textContent || '', 200);
    if (!text) continue;
    headings.push(heading.tagName.toLowerCase() + ': ' + text);
  }

  var forms = [];
  var formNodes = document.querySelectorAll('form');
  var formLimit = mode === 'compact' ? 3 : 10;
  var fieldLimit = mode === 'compact' ? 5 : 25;
  for (var k = 0; k < formNodes.length && forms.length < formLimit; k++) {
    var form = formNodes[k];
    var fieldNodes = form.querySelectorAll('input, select, textarea');
    var fields = [];
    var seenFields = {};
    for (var m = 0; m < fieldNodes.length && fields.length < fieldLimit; m++) {
      var field = fieldNodes[m];
      var candidate =
        field.getAttribute('name') ||
        field.getAttribute('id') ||
        field.getAttribute('aria-label') ||
        field.getAttribute('type') ||
        field.tagName.toLowerCase();
      candidate = cleanText(candidate, 60);
      if (!candidate || seenFields[candidate]) continue;
      seenFields[candidate] = true;
      fields.push(candidate);
    }
    forms.push({
      action: absoluteHref(form.getAttribute('action') || window.location.href),
      method: (form.getAttribute('method') || 'GET').toUpperCase(),
      fields: fields
    });
  }

  var mainNode = findMainNode();
  var mainText = cleanText(mainNode ? mainNode.innerText || mainNode.textContent || '' : '', 20000);
  var previewLimit = mode === 'compact' ? 300 : 500;
  var preview = mainText.slice(0, previewLimit);

  var interactiveCandidates = (mainNode || document).querySelectorAll('button, [role="button"], [onclick], a[href]');
  var interactiveCount = visibleInteractiveCount();
  var linkCount = document.querySelectorAll('a[href]').length;
  var paragraphCount = document.querySelectorAll('p').length;
  var headingCount = headingNodes.length;
  var pageType = classifyPage(forms, interactiveCount, linkCount, paragraphCount, headingCount, preview);

  if (mode === 'compact') {
    return {
      type: pageType,
      title: cleanText(document.title, 120),
      headings: headings,
      primary_actions: extractPrimaryActions(interactiveCandidates),
      forms: forms,
      content_preview: preview,
      interactive_count: interactiveCount
    };
  }

  // Full mode (standalone analyze)
  var navCandidates = document.querySelectorAll('nav a[href], header a[href], [role="navigation"] a[href]');
  var navLinks = [];
  var seenNav = {};
  for (var j = 0; j < navCandidates.length && navLinks.length < 25; j++) {
    var link = navCandidates[j];
    var linkText = cleanText(link.innerText || link.textContent || '', 80);
    var href = absoluteHref(link.getAttribute('href'));
    if (!href) continue;
    var key = linkText + '|' + href;
    if (seenNav[key]) continue;
    seenNav[key] = true;
    navLinks.push({ text: linkText, href: href });
  }

  return {
    url: window.location.href,
    title: document.title || '',
    type: pageType,
    headings: headings,
    nav_links: navLinks,
    forms: forms,
    interactive_element_count: interactiveCount,
    main_content_preview: preview,
    word_count: mainText ? mainText.split(/\s+/).filter(Boolean).length : 0
  };
})`

// compactSummaryScript returns the summary script with mode='compact'.
func compactSummaryScript() string {
	return fmt.Sprintf("%s('compact')", pageSummaryScript)
}

// fullSummaryScript returns the summary script with mode='full'.
func fullSummaryScript() string {
	return fmt.Sprintf("%s('full')", pageSummaryScript)
}
