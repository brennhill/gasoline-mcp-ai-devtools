// csp.go — CSP and SRI generation from captured network traffic.
package generate

import (
	"strings"

	"github.com/dev-console/dev-console/internal/capture"
)

// BuildCSPDirectives extracts unique origins from network bodies and groups them by CSP directive.
func BuildCSPDirectives(networkBodies []capture.NetworkBody) map[string][]string {
	originsByType := make(map[string]map[string]bool)
	for _, body := range networkBodies {
		origin := ExtractOrigin(body.URL)
		if origin == "" {
			continue
		}
		directive := resourceTypeToCSPDirective(body.ContentType)
		if originsByType[directive] == nil {
			originsByType[directive] = make(map[string]bool)
		}
		originsByType[directive][origin] = true
	}

	directives := map[string][]string{"default-src": {"'self'"}}
	for directive, origins := range originsByType {
		originList := make([]string, 0, len(origins))
		for origin := range origins {
			originList = append(originList, origin)
		}
		if len(originList) > 0 {
			directives[directive] = append([]string{"'self'"}, originList...)
		}
	}
	return directives
}

// BuildCSPPolicyString serializes CSP directives into a semicolon-separated policy string.
func BuildCSPPolicyString(directives map[string][]string) string {
	var policyParts []string
	for directive, sources := range directives {
		policyParts = append(policyParts, directive+" "+strings.Join(sources, " "))
	}
	return strings.Join(policyParts, "; ")
}

// ExtractOrigin extracts the origin (scheme://host:port) from a URL.
func ExtractOrigin(urlStr string) string {
	if urlStr == "" {
		return ""
	}
	idx := 0
	if len(urlStr) > 8 && urlStr[:8] == "https://" {
		idx = 8
	} else if len(urlStr) > 7 && urlStr[:7] == "http://" {
		idx = 7
	} else {
		return ""
	}
	endIdx := idx
	for endIdx < len(urlStr) && urlStr[endIdx] != '/' && urlStr[endIdx] != '?' {
		endIdx++
	}
	return urlStr[:endIdx]
}

// resourceTypeToCSPDirective maps content-type to CSP directive.
func resourceTypeToCSPDirective(contentType string) string {
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "javascript"):
		return "script-src"
	case strings.Contains(ct, "css"):
		return "style-src"
	case strings.Contains(ct, "font"):
		return "font-src"
	case strings.Contains(ct, "image"):
		return "img-src"
	case strings.Contains(ct, "video"), strings.Contains(ct, "audio"):
		return "media-src"
	default:
		return "connect-src"
	}
}
