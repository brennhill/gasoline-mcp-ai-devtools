// tools_interact_content.go — Content extraction handlers for interact tool.
// Implements get_readable and get_markdown actions using embedded JS scripts
// executed via the "execute" query type (no TypeScript changes needed).
package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
)

// getReadableScript is a self-contained IIFE that extracts clean article content.
// Finds <article>/<main>/largest text block, strips nav/footer/ads, returns structured content.
const getReadableScript = `(function() {
  function cleanText(el) {
    if (!el) return '';
    var clone = el.cloneNode(true);
    var removeTags = ['nav','header','footer','aside','script','style','noscript','svg',
      '[role="navigation"]','[role="banner"]','[role="contentinfo"]','[aria-hidden="true"]',
      '.ad,.ads,.advertisement,.social-share,.comments,.sidebar,.related-posts,.newsletter'];
    removeTags.forEach(function(sel) {
      var els = clone.querySelectorAll(sel);
      for (var i = 0; i < els.length; i++) els[i].remove();
    });
    return (clone.innerText || clone.textContent || '').replace(/\s+/g,' ').trim();
  }

  function findMainContent() {
    var candidates = ['article','main','[role="main"]','.post-content','.entry-content',
      '.article-body','.article-content','.story-body','#content','.content'];
    for (var i = 0; i < candidates.length; i++) {
      var el = document.querySelector(candidates[i]);
      if (el) {
        var text = cleanText(el);
        if (text.length > 100) return {el: el, text: text};
      }
    }
    return {el: document.body, text: cleanText(document.body)};
  }

  function getByline() {
    var selectors = ['.author','[rel="author"]','.byline','.post-author','meta[name="author"]'];
    for (var i = 0; i < selectors.length; i++) {
      var el = document.querySelector(selectors[i]);
      if (el) {
        var text = el.getAttribute('content') || el.innerText || '';
        text = text.trim();
        if (text.length > 0 && text.length < 200) return text;
      }
    }
    return '';
  }

  var found = findMainContent();
  var content = found.text;
  var excerpt = content.slice(0, 300);
  var words = content.split(/\s+/).filter(Boolean);

  return {
    title: document.title || '',
    content: content,
    excerpt: excerpt,
    byline: getByline(),
    word_count: words.length,
    url: window.location.href
  };
})()`;

// getMarkdownScript extracts content and converts to Markdown.
const getMarkdownScript = `(function() {
  function findMainContent() {
    var candidates = ['article','main','[role="main"]','.post-content','.entry-content',
      '.article-body','.article-content','.story-body','#content','.content'];
    for (var i = 0; i < candidates.length; i++) {
      var el = document.querySelector(candidates[i]);
      if (el && (el.innerText||'').trim().length > 100) return el;
    }
    return document.body;
  }

  function nodeToMarkdown(node, depth) {
    if (!node) return '';
    if (depth > 20) return '';
    if (node.nodeType === 3) return node.textContent || '';
    if (node.nodeType !== 1) return '';
    var el = node;
    var tag = el.tagName.toLowerCase();

    // Skip unwanted elements
    if (['nav','header','footer','aside','script','style','noscript','svg'].indexOf(tag) >= 0) return '';
    if (el.getAttribute('role') === 'navigation') return '';
    if (el.getAttribute('aria-hidden') === 'true') return '';

    var children = '';
    for (var i = 0; i < el.childNodes.length; i++) {
      children += nodeToMarkdown(el.childNodes[i], depth + 1);
    }
    children = children.replace(/\n{3,}/g, '\n\n');

    switch(tag) {
      case 'h1': return '\n# ' + children.trim() + '\n\n';
      case 'h2': return '\n## ' + children.trim() + '\n\n';
      case 'h3': return '\n### ' + children.trim() + '\n\n';
      case 'h4': return '\n#### ' + children.trim() + '\n\n';
      case 'h5': return '\n##### ' + children.trim() + '\n\n';
      case 'h6': return '\n###### ' + children.trim() + '\n\n';
      case 'p': return '\n' + children.trim() + '\n\n';
      case 'br': return '\n';
      case 'hr': return '\n---\n\n';
      case 'strong': case 'b': return '**' + children.trim() + '**';
      case 'em': case 'i': return '*' + children.trim() + '*';
      case 'code': return '` + "`" + `' + children.trim() + '` + "`" + `';
      case 'pre': return '\n` + "```" + `\n' + (el.innerText||'').trim() + '\n` + "```" + `\n\n';
      case 'a':
        var href = el.getAttribute('href') || '';
        if (href && href !== '#' && !href.startsWith('javascript:')) {
          try { href = new URL(href, window.location.href).href; } catch(e) {}
          return '[' + children.trim() + '](' + href + ')';
        }
        return children;
      case 'img':
        var src = el.getAttribute('src') || '';
        var alt = el.getAttribute('alt') || '';
        if (src) {
          try { src = new URL(src, window.location.href).href; } catch(e) {}
          return '![' + alt + '](' + src + ')';
        }
        return '';
      case 'ul': case 'ol': return '\n' + children + '\n';
      case 'li':
        var parent = el.parentElement;
        if (parent && parent.tagName.toLowerCase() === 'ol') {
          var idx = Array.from(parent.children).indexOf(el) + 1;
          return idx + '. ' + children.trim() + '\n';
        }
        return '- ' + children.trim() + '\n';
      case 'blockquote': return '\n> ' + children.trim().replace(/\n/g, '\n> ') + '\n\n';
      case 'table': return '\n' + tableToMarkdown(el) + '\n\n';
      case 'div': case 'section': case 'article': case 'main': return children;
      default: return children;
    }
  }

  function tableToMarkdown(table) {
    var rows = table.querySelectorAll('tr');
    if (rows.length === 0) return '';
    var md = '';
    for (var r = 0; r < rows.length; r++) {
      var cells = rows[r].querySelectorAll('th,td');
      var row = '|';
      for (var c = 0; c < cells.length; c++) {
        row += ' ' + (cells[c].innerText||'').trim().replace(/\|/g,'\\|').replace(/\n/g,' ') + ' |';
      }
      md += row + '\n';
      if (r === 0 && rows[r].querySelector('th')) {
        md += '|';
        for (var c2 = 0; c2 < cells.length; c2++) md += ' --- |';
        md += '\n';
      }
    }
    return md;
  }

  var main = findMainContent();
  var markdown = nodeToMarkdown(main, 0).trim();
  var words = markdown.replace(/[#*\[\]()` + "`" + `|>-]/g,' ').split(/\s+/).filter(Boolean);

  return {
    title: document.title || '',
    markdown: markdown,
    word_count: words.length,
    url: window.location.href
  };
})()`;

func (h *ToolHandler) handleGetReadable(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID     int    `json:"tab_id,omitempty"`
		TimeoutMs int    `json:"timeout_ms,omitempty"`
		World     string `json:"world,omitempty"`
	}
	lenientUnmarshal(args, &params)

	if params.World == "" {
		params.World = "isolated"
	}
	if !validWorldValues[params.World] {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Invalid 'world' value: "+params.World, "Use 'isolated' (default), 'main', or 'auto'", withParam("world"))}
	}
	if params.TimeoutMs <= 0 {
		params.TimeoutMs = 10_000
	}

	correlationID := fmt.Sprintf("readable_%d_%d", time.Now().UnixNano(), randomInt63())
	execParams, _ := json.Marshal(map[string]any{
		"script":     getReadableScript,
		"timeout_ms": params.TimeoutMs,
		"world":      params.World,
		"reason":     "get_readable",
	})

	query := queries.PendingQuery{
		Type:          "execute",
		Params:        execParams,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return h.maybeWaitForCommand(req, correlationID, args, "get_readable queued")
}

func (h *ToolHandler) handleGetMarkdown(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID     int    `json:"tab_id,omitempty"`
		TimeoutMs int    `json:"timeout_ms,omitempty"`
		World     string `json:"world,omitempty"`
	}
	lenientUnmarshal(args, &params)

	if params.World == "" {
		params.World = "isolated"
	}
	if !validWorldValues[params.World] {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Invalid 'world' value: "+params.World, "Use 'isolated' (default), 'main', or 'auto'", withParam("world"))}
	}
	if params.TimeoutMs <= 0 {
		params.TimeoutMs = 10_000
	}

	correlationID := fmt.Sprintf("markdown_%d_%d", time.Now().UnixNano(), randomInt63())
	execParams, _ := json.Marshal(map[string]any{
		"script":     getMarkdownScript,
		"timeout_ms": params.TimeoutMs,
		"world":      params.World,
		"reason":     "get_markdown",
	})

	query := queries.PendingQuery{
		Type:          "execute",
		Params:        execParams,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return h.maybeWaitForCommand(req, correlationID, args, "get_markdown queued")
}

// enrichNavigateResponse appends page content to a successful navigate response.
// Runs a page_summary script to extract text content, headings, and metadata.
func (h *ToolHandler) enrichNavigateResponse(resp JSONRPCResponse, req JSONRPCRequest, tabID int) JSONRPCResponse {
	// Only enrich successful (non-error) responses
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || result.IsError {
		return resp
	}

	// Get current page info from tracking state
	_, _, tabURL := h.capture.GetTrackingStatus()
	tabTitle := h.capture.GetTrackedTabTitle()

	// Get performance vitals
	vitals := h.capture.GetPerformanceSnapshots()

	// Execute page summary script for text content
	summaryCorrelationID := fmt.Sprintf("nav_content_%d_%d", time.Now().UnixNano(), randomInt63())
	execParams, _ := json.Marshal(map[string]any{
		"script":     pageSummaryScript,
		"timeout_ms": 10000,
		"world":      "isolated",
		"reason":     "navigate_content_enrichment",
	})
	summaryQuery := queries.PendingQuery{
		Type:          "execute",
		Params:        execParams,
		TabID:         tabID,
		CorrelationID: summaryCorrelationID,
	}
	h.capture.CreatePendingQueryWithTimeout(summaryQuery, queries.AsyncCommandTimeout, req.ClientID)

	// Wait for page summary (5s timeout — page should already be loaded)
	var textContent string
	cmd, found := h.capture.WaitForCommand(summaryCorrelationID, 5*time.Second)
	if found && cmd.Status != "pending" && cmd.Result != nil {
		var summaryResult map[string]any
		if json.Unmarshal(cmd.Result, &summaryResult) == nil {
			if preview, ok := summaryResult["main_content_preview"].(string); ok {
				textContent = preview
			}
		}
	}

	// Parse existing result text and append enrichment data
	if len(result.Content) > 0 {
		enrichment := map[string]any{
			"url":          tabURL,
			"title":        tabTitle,
			"text_content": textContent,
		}
		if len(vitals) > 0 {
			enrichment["vitals"] = vitals[len(vitals)-1]
		}
		enrichJSON, _ := json.Marshal(enrichment)
		result.Content = append(result.Content, MCPContentBlock{
			Type: "text",
			Text: "Page content:\n" + string(enrichJSON),
		})
	}

	resultJSON, _ := json.Marshal(result)
	resp.Result = json.RawMessage(resultJSON)
	return resp
}
