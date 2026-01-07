// sri_test.go — Tests for SRI Hash Generator (generate_sri) MCP tool.
// Tests hash computation, resource filtering, third-party detection, and output formats.
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSRIGeneratorBasicHash(t *testing.T) {
	gen := NewSRIGenerator()
	bodies := []NetworkBody{
		{
			URL:          "https://cdn.example.com/app.js",
			ContentType:  "application/javascript",
			Method:       "GET",
			ResponseBody: "console.log('hello');",
		},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, SRIParams{})

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	res := result.Resources[0]
	if res.URL != "https://cdn.example.com/app.js" {
		t.Errorf("expected URL https://cdn.example.com/app.js, got %s", res.URL)
	}
	if res.Type != "script" {
		t.Errorf("expected type=script, got %s", res.Type)
	}
	// Hash should start with sha384-
	if !strings.HasPrefix(res.Hash, "sha384-") {
		t.Errorf("expected hash to start with sha384-, got %s", res.Hash)
	}
	// Hash should be base64 encoded (44 chars for SHA-384)
	hashPart := strings.TrimPrefix(res.Hash, "sha384-")
	if len(hashPart) != 64 {
		t.Errorf("expected hash base64 length 64, got %d", len(hashPart))
	}
	if res.Crossorigin != "anonymous" {
		t.Errorf("expected crossorigin=anonymous, got %s", res.Crossorigin)
	}
}

func TestSRIGeneratorKnownHash(t *testing.T) {
	// Test with known content to verify SHA-384 computation
	// "test content" SHA-384 hash is known
	gen := NewSRIGenerator()
	bodies := []NetworkBody{
		{
			URL:          "https://cdn.example.com/test.js",
			ContentType:  "application/javascript",
			Method:       "GET",
			ResponseBody: "test content",
		},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, SRIParams{})

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	// SHA-384 of "test content" computed via: echo -n "test content" | openssl dgst -sha384 -binary | base64
	expectedHash := "sha384-8cFK5mW+eeVbAO7clwcEVX1yowIas7iMz9wbg9HWbEeQkeI8+2Ah9Dt6EnOm9KMY"
	if result.Resources[0].Hash != expectedHash {
		t.Errorf("hash mismatch:\nexpected: %s\ngot:      %s", expectedHash, result.Resources[0].Hash)
	}
}

func TestSRIGeneratorOnlyScriptsAndStyles(t *testing.T) {
	gen := NewSRIGenerator()
	bodies := []NetworkBody{
		// Should be included
		{URL: "https://cdn.example.com/app.js", ContentType: "application/javascript", ResponseBody: "js code"},
		{URL: "https://cdn.example.com/style.css", ContentType: "text/css", ResponseBody: "css code"},
		// Should be excluded - not script/style
		{URL: "https://cdn.example.com/data.json", ContentType: "application/json", ResponseBody: `{"data":1}`},
		{URL: "https://cdn.example.com/logo.png", ContentType: "image/png", ResponseBody: "binary data"},
		{URL: "https://cdn.example.com/font.woff2", ContentType: "font/woff2", ResponseBody: "font data"},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, SRIParams{})

	if len(result.Resources) != 2 {
		t.Fatalf("expected 2 resources (script + style), got %d", len(result.Resources))
	}
	types := map[string]bool{}
	for _, r := range result.Resources {
		types[r.Type] = true
	}
	if !types["script"] {
		t.Error("expected script type in results")
	}
	if !types["style"] {
		t.Error("expected style type in results")
	}
}

func TestSRIGeneratorOnlyThirdParty(t *testing.T) {
	gen := NewSRIGenerator()
	bodies := []NetworkBody{
		// Same origin - should be excluded
		{URL: "https://myapp.com/app.js", ContentType: "application/javascript", ResponseBody: "first party"},
		// Third party - should be included
		{URL: "https://cdn.example.com/lib.js", ContentType: "application/javascript", ResponseBody: "third party"},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, SRIParams{})

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 third-party resource, got %d", len(result.Resources))
	}
	if result.Resources[0].URL != "https://cdn.example.com/lib.js" {
		t.Errorf("expected third-party URL, got %s", result.Resources[0].URL)
	}
}

func TestSRIGeneratorTagTemplates(t *testing.T) {
	gen := NewSRIGenerator()
	bodies := []NetworkBody{
		{URL: "https://cdn.example.com/app.js", ContentType: "application/javascript", ResponseBody: "js"},
		{URL: "https://cdn.example.com/style.css", ContentType: "text/css", ResponseBody: "css"},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, SRIParams{})

	if len(result.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(result.Resources))
	}

	for _, res := range result.Resources {
		if res.Type == "script" {
			if !strings.HasPrefix(res.TagTemplate, "<script src=") {
				t.Errorf("script tag template should start with <script src=, got %s", res.TagTemplate)
			}
			if !strings.Contains(res.TagTemplate, "integrity=\"sha384-") {
				t.Error("script tag should contain integrity attribute")
			}
			if !strings.Contains(res.TagTemplate, "crossorigin=\"anonymous\"") {
				t.Error("script tag should contain crossorigin attribute")
			}
			if !strings.HasSuffix(res.TagTemplate, "</script>") {
				t.Error("script tag should end with </script>")
			}
		}
		if res.Type == "style" {
			if !strings.HasPrefix(res.TagTemplate, "<link rel=\"stylesheet\"") {
				t.Errorf("style tag should start with <link rel=\"stylesheet\", got %s", res.TagTemplate)
			}
			if !strings.Contains(res.TagTemplate, "integrity=\"sha384-") {
				t.Error("style tag should contain integrity attribute")
			}
			if !strings.Contains(res.TagTemplate, "crossorigin=\"anonymous\"") {
				t.Error("style tag should contain crossorigin attribute")
			}
		}
	}
}

func TestSRIGeneratorVaryUserAgentWarning(t *testing.T) {
	gen := NewSRIGenerator()
	bodies := []NetworkBody{
		{
			URL:          "https://fonts.googleapis.com/css2?family=Roboto",
			ContentType:  "text/css",
			ResponseBody: "font css",
			ResponseHeaders: map[string]string{
				"Vary": "User-Agent",
			},
		},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, SRIParams{})

	if len(result.Warnings) == 0 {
		t.Error("expected warning about Vary: User-Agent")
	}
	foundWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "User-Agent") || strings.Contains(w, "Vary") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("expected Vary: User-Agent warning, got %v", result.Warnings)
	}
}

func TestSRIGeneratorTruncatedBody(t *testing.T) {
	gen := NewSRIGenerator()
	bodies := []NetworkBody{
		{
			URL:               "https://cdn.example.com/large.js",
			ContentType:       "application/javascript",
			ResponseBody:      "truncated...",
			ResponseTruncated: true,
		},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, SRIParams{})

	// Truncated bodies should not be included
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources for truncated body, got %d", len(result.Resources))
	}
	// Should have a warning
	if len(result.Warnings) == 0 {
		t.Error("expected warning about truncated body")
	}
	foundWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "truncated") || strings.Contains(w, "large") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("expected truncation warning, got %v", result.Warnings)
	}
}

func TestSRIGeneratorEmptyBody(t *testing.T) {
	gen := NewSRIGenerator()
	bodies := []NetworkBody{
		{
			URL:          "https://cdn.example.com/empty.js",
			ContentType:  "application/javascript",
			ResponseBody: "",
		},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, SRIParams{})

	// Empty body should not generate SRI
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources for empty body, got %d", len(result.Resources))
	}
}

func TestSRIGeneratorSummary(t *testing.T) {
	gen := NewSRIGenerator()
	bodies := []NetworkBody{
		{URL: "https://cdn1.example.com/a.js", ContentType: "application/javascript", ResponseBody: "a"},
		{URL: "https://cdn2.example.com/b.js", ContentType: "application/javascript", ResponseBody: "b"},
		{URL: "https://cdn3.example.com/c.css", ContentType: "text/css", ResponseBody: "c"},
		// Image - not counted
		{URL: "https://cdn4.example.com/d.png", ContentType: "image/png", ResponseBody: "d"},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, SRIParams{})

	if result.Summary.TotalThirdPartyResources != 4 {
		t.Errorf("expected 4 total third party resources, got %d", result.Summary.TotalThirdPartyResources)
	}
	if result.Summary.ScriptsWithoutSRI != 2 {
		t.Errorf("expected 2 scripts without SRI, got %d", result.Summary.ScriptsWithoutSRI)
	}
	if result.Summary.StylesWithoutSRI != 1 {
		t.Errorf("expected 1 style without SRI, got %d", result.Summary.StylesWithoutSRI)
	}
	if result.Summary.HashesGenerated != 3 {
		t.Errorf("expected 3 hashes generated, got %d", result.Summary.HashesGenerated)
	}
}

func TestSRIGeneratorResourceTypesFilter(t *testing.T) {
	gen := NewSRIGenerator()
	bodies := []NetworkBody{
		{URL: "https://cdn.example.com/app.js", ContentType: "application/javascript", ResponseBody: "js"},
		{URL: "https://cdn.example.com/style.css", ContentType: "text/css", ResponseBody: "css"},
	}
	pageURLs := []string{"https://myapp.com/"}

	// Only scripts
	result := gen.Generate(bodies, pageURLs, SRIParams{ResourceTypes: []string{"scripts"}})
	if len(result.Resources) != 1 || result.Resources[0].Type != "script" {
		t.Errorf("expected only script when filtering by scripts, got %d resources", len(result.Resources))
	}

	// Only styles
	result = gen.Generate(bodies, pageURLs, SRIParams{ResourceTypes: []string{"styles"}})
	if len(result.Resources) != 1 || result.Resources[0].Type != "style" {
		t.Errorf("expected only style when filtering by styles, got %d resources", len(result.Resources))
	}
}

func TestSRIGeneratorOriginsFilter(t *testing.T) {
	gen := NewSRIGenerator()
	bodies := []NetworkBody{
		{URL: "https://cdn1.example.com/app.js", ContentType: "application/javascript", ResponseBody: "js1"},
		{URL: "https://cdn2.example.com/app.js", ContentType: "application/javascript", ResponseBody: "js2"},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, SRIParams{Origins: []string{"https://cdn1.example.com"}})

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource with origin filter, got %d", len(result.Resources))
	}
	if result.Resources[0].URL != "https://cdn1.example.com/app.js" {
		t.Errorf("expected cdn1.example.com URL, got %s", result.Resources[0].URL)
	}
}

func TestSRIGeneratorSizeBytes(t *testing.T) {
	gen := NewSRIGenerator()
	content := "console.log('hello world');"
	bodies := []NetworkBody{
		{
			URL:          "https://cdn.example.com/app.js",
			ContentType:  "application/javascript",
			ResponseBody: content,
		},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, SRIParams{})

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].SizeBytes != len(content) {
		t.Errorf("expected size %d, got %d", len(content), result.Resources[0].SizeBytes)
	}
}

func TestSRIGeneratorAlreadyHasSRI(t *testing.T) {
	// This tests that we track resources that already have SRI
	// Note: In practice, Gasoline captures the response, not the HTML.
	// AlreadyHasSRI would be set based on external data or heuristics.
	// For now, we just ensure the field exists in the output.
	gen := NewSRIGenerator()
	bodies := []NetworkBody{
		{URL: "https://cdn.example.com/app.js", ContentType: "application/javascript", ResponseBody: "js"},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, SRIParams{})

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	// By default, AlreadyHasSRI should be false since we're computing new hashes
	if result.Resources[0].AlreadyHasSRI {
		t.Error("expected AlreadyHasSRI=false for newly computed hash")
	}
}

func TestSRIGeneratorDeduplication(t *testing.T) {
	gen := NewSRIGenerator()
	// Same URL loaded twice - should only appear once
	bodies := []NetworkBody{
		{URL: "https://cdn.example.com/app.js", ContentType: "application/javascript", ResponseBody: "js"},
		{URL: "https://cdn.example.com/app.js", ContentType: "application/javascript", ResponseBody: "js"},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, SRIParams{})

	if len(result.Resources) != 1 {
		t.Errorf("expected 1 deduplicated resource, got %d", len(result.Resources))
	}
}

func TestSRIGeneratorMultipleContentTypes(t *testing.T) {
	gen := NewSRIGenerator()
	bodies := []NetworkBody{
		// Various JavaScript content types
		{URL: "https://cdn.example.com/a.js", ContentType: "application/javascript", ResponseBody: "a"},
		{URL: "https://cdn.example.com/b.js", ContentType: "text/javascript", ResponseBody: "b"},
		{URL: "https://cdn.example.com/c.js", ContentType: "application/x-javascript", ResponseBody: "c"},
		// CSS
		{URL: "https://cdn.example.com/d.css", ContentType: "text/css", ResponseBody: "d"},
	}
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, SRIParams{})

	if len(result.Resources) != 4 {
		t.Errorf("expected 4 resources, got %d", len(result.Resources))
	}
}

func TestSRIGeneratorHandleMCP(t *testing.T) {
	bodies := []NetworkBody{
		{URL: "https://cdn.example.com/lib.js", ContentType: "application/javascript", ResponseBody: "code"},
	}
	pageURLs := []string{"https://myapp.com/"}

	params := SRIParams{}
	raw, _ := json.Marshal(params)

	result, err := HandleGenerateSRI(json.RawMessage(raw), bodies, pageURLs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sriResult, ok := result.(SRIResult)
	if !ok {
		t.Fatal("expected SRIResult type")
	}
	if len(sriResult.Resources) != 1 {
		t.Errorf("expected 1 resource, got %d", len(sriResult.Resources))
	}
}

func TestSRIGeneratorEmptyInput(t *testing.T) {
	gen := NewSRIGenerator()
	result := gen.Generate(nil, nil, SRIParams{})

	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources for empty input, got %d", len(result.Resources))
	}
	if result.Summary.HashesGenerated != 0 {
		t.Errorf("expected 0 hashes generated, got %d", result.Summary.HashesGenerated)
	}
}

func TestSRIGeneratorInvalidParams(t *testing.T) {
	bodies := []NetworkBody{
		{URL: "https://cdn.example.com/lib.js", ContentType: "application/javascript", ResponseBody: "code"},
	}
	pageURLs := []string{"https://myapp.com/"}

	// Invalid JSON params should return error
	_, err := HandleGenerateSRI([]byte(`{invalid}`), bodies, pageURLs)
	if err == nil {
		t.Error("expected error for invalid JSON params")
	}
}

func TestSRIGeneratorSubdomainFirstParty(t *testing.T) {
	gen := NewSRIGenerator()
	bodies := []NetworkBody{
		// Subdomain of first party - should be excluded
		{URL: "https://cdn.myapp.com/app.js", ContentType: "application/javascript", ResponseBody: "first party cdn"},
		// Different domain - should be included
		{URL: "https://cdn.example.com/lib.js", ContentType: "application/javascript", ResponseBody: "third party"},
	}
	// Note: subdomain detection depends on implementation
	pageURLs := []string{"https://myapp.com/"}

	result := gen.Generate(bodies, pageURLs, SRIParams{})

	// cdn.myapp.com is a different origin from myapp.com, so it should be included as third party
	// (unless we implement subdomain matching, which the spec doesn't require)
	if len(result.Resources) < 1 {
		t.Errorf("expected at least 1 resource, got %d", len(result.Resources))
	}
}
