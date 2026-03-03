// Purpose: Computes SHA-384 hashes and provides URL/origin helper functions for SRI generation.
// Why: Isolates cryptographic and URL utilities from SRI generation and tooling logic.
package security

import (
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

// computeSHA384 computes the SHA-384 hash of content and returns it in SRI format.
func computeSHA384(content string) string {
	hasher := sha512.New384()
	hasher.Write([]byte(content))
	hash := hasher.Sum(nil)
	b64 := base64.StdEncoding.EncodeToString(hash)
	return "sha384-" + b64
}

// sriResourceType returns "script" or "style" based on content type, or empty string if not applicable.
func sriResourceType(contentType string) string {
	ct := strings.ToLower(contentType)

	// Strip parameters (e.g., "text/css; charset=utf-8" -> "text/css")
	if idx := strings.IndexByte(ct, ';'); idx >= 0 {
		ct = ct[:idx]
	}
	ct = strings.TrimSpace(ct)

	// JavaScript types
	if strings.Contains(ct, "javascript") {
		return "script"
	}

	// CSS
	if ct == "text/css" {
		return "style"
	}

	return ""
}

// extractOriginForSRI delegates to util.ExtractOrigin for URL origin extraction.
func extractOriginForSRI(rawURL string) string {
	return util.ExtractOrigin(rawURL)
}

// generateTagTemplate creates an HTML tag with SRI attributes.
func generateTagTemplate(resourceURL, hash, resType string) string {
	if resType == "script" {
		return fmt.Sprintf(`<script src="%s" integrity="%s" crossorigin="anonymous"></script>`, resourceURL, hash)
	}
	if resType == "style" {
		return fmt.Sprintf(`<link rel="stylesheet" href="%s" integrity="%s" crossorigin="anonymous">`, resourceURL, hash)
	}
	return ""
}
