// Purpose: Derives coarse auth pattern hints from observed endpoint traffic.
// Why: Keeps auth inference heuristics independent from schema and OpenAPI formatting logic.
// Docs: docs/features/feature/api-schema/index.md

package analysis

import "strings"

// ============================================
// Auth Pattern Detection
// ============================================

func (s *SchemaStore) detectAuthPattern() *AuthPattern {
	hasAuthEndpoint := false
	has401 := false
	var publicPaths []string

	authKeywords := []string{"/auth", "/login", "/token"}
	publicKeywords := []string{"/health", "/public"}

	for _, acc := range s.accumulators {
		lowerPattern := strings.ToLower(acc.pathPattern)
		if containsAny(lowerPattern, authKeywords) {
			hasAuthEndpoint = true
			publicPaths = append(publicPaths, acc.pathPattern)
		}
		if containsAny(lowerPattern, publicKeywords) {
			publicPaths = append(publicPaths, acc.pathPattern)
		}
		if !has401 {
			has401 = hasStatusCode(acc.responseShapes, 401)
		}
	}

	if !hasAuthEndpoint && !has401 {
		return nil
	}
	return &AuthPattern{Type: "bearer", Header: "Authorization", AuthRate: 100.0, PublicPaths: publicPaths}
}

// containsAny returns true if s contains any of the substrings.
func containsAny(s string, substrings []string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// hasStatusCode checks if a response shapes map contains the given status code.
func hasStatusCode(shapes map[int]*responseAccumulator, code int) bool {
	_, ok := shapes[code]
	return ok
}
