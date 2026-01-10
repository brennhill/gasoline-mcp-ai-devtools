package main

import (
	"testing"
)

// ============================================
// mapToOpenAPIType Coverage Tests
// ============================================

func TestMapToOpenAPITypeInteger(t *testing.T) {
	t.Parallel()
	result := mapToOpenAPIType("integer")
	if result != "integer" {
		t.Errorf("Expected 'integer', got: %s", result)
	}
}

func TestMapToOpenAPITypeNumber(t *testing.T) {
	t.Parallel()
	result := mapToOpenAPIType("number")
	if result != "number" {
		t.Errorf("Expected 'number', got: %s", result)
	}
}

func TestMapToOpenAPITypeBoolean(t *testing.T) {
	t.Parallel()
	result := mapToOpenAPIType("boolean")
	if result != "boolean" {
		t.Errorf("Expected 'boolean', got: %s", result)
	}
}

func TestMapToOpenAPITypeArray(t *testing.T) {
	t.Parallel()
	result := mapToOpenAPIType("array")
	if result != "array" {
		t.Errorf("Expected 'array', got: %s", result)
	}
}

func TestMapToOpenAPITypeObject(t *testing.T) {
	t.Parallel()
	result := mapToOpenAPIType("object")
	if result != "object" {
		t.Errorf("Expected 'object', got: %s", result)
	}
}

func TestMapToOpenAPITypeUUID(t *testing.T) {
	t.Parallel()
	result := mapToOpenAPIType("uuid")
	if result != "string" {
		t.Errorf("Expected 'string' for uuid, got: %s", result)
	}
}

func TestMapToOpenAPITypeDefault(t *testing.T) {
	t.Parallel()
	result := mapToOpenAPIType("something_unknown")
	if result != "string" {
		t.Errorf("Expected 'string' for default, got: %s", result)
	}
}

func TestMapToOpenAPITypeEmptyString(t *testing.T) {
	t.Parallel()
	result := mapToOpenAPIType("")
	if result != "string" {
		t.Errorf("Expected 'string' for empty input, got: %s", result)
	}
}

// ============================================
// intToString Coverage Tests
// ============================================

func TestIntToStringZero(t *testing.T) {
	t.Parallel()
	result := intToString(0)
	if result != "0" {
		t.Errorf("Expected '0', got: %s", result)
	}
}

func TestIntToStringPositive(t *testing.T) {
	t.Parallel()
	result := intToString(200)
	if result != "200" {
		t.Errorf("Expected '200', got: %s", result)
	}
}

func TestIntToStringLargePositive(t *testing.T) {
	t.Parallel()
	result := intToString(12345)
	if result != "12345" {
		t.Errorf("Expected '12345', got: %s", result)
	}
}

func TestIntToStringNegative(t *testing.T) {
	t.Parallel()
	result := intToString(-42)
	if result != "-42" {
		t.Errorf("Expected '-42', got: %s", result)
	}
}

func TestIntToStringLargeNegative(t *testing.T) {
	t.Parallel()
	result := intToString(-999)
	if result != "-999" {
		t.Errorf("Expected '-999', got: %s", result)
	}
}

func TestIntToStringSingleDigit(t *testing.T) {
	t.Parallel()
	result := intToString(7)
	if result != "7" {
		t.Errorf("Expected '7', got: %s", result)
	}
}

func TestIntToStringStatusCodes(t *testing.T) {
	t.Parallel()
	// Test common HTTP status codes that might be used in the codebase
	tests := []struct {
		input    int
		expected string
	}{
		{200, "200"},
		{201, "201"},
		{301, "301"},
		{400, "400"},
		{401, "401"},
		{403, "403"},
		{404, "404"},
		{500, "500"},
		{502, "502"},
		{503, "503"},
	}
	for _, tt := range tests {
		result := intToString(tt.input)
		if result != tt.expected {
			t.Errorf("intToString(%d) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

// ============================================
// buildPathParams Coverage Tests
// ============================================

func TestBuildPathParamsUUID(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()
	params := store.buildPathParams("/api/users/{uuid}")

	if len(params) != 1 {
		t.Fatalf("Expected 1 param, got %d", len(params))
	}
	if params[0].Name != "uuid" {
		t.Errorf("Expected name 'uuid', got: %s", params[0].Name)
	}
	if params[0].Type != "uuid" {
		t.Errorf("Expected type 'uuid', got: %s", params[0].Type)
	}
	if params[0].Position != 3 {
		t.Errorf("Expected position 3, got: %d", params[0].Position)
	}
}

func TestBuildPathParamsID(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()
	params := store.buildPathParams("/api/items/{id}")

	if len(params) != 1 {
		t.Fatalf("Expected 1 param, got %d", len(params))
	}
	if params[0].Name != "id" {
		t.Errorf("Expected name 'id', got: %s", params[0].Name)
	}
	if params[0].Type != "integer" {
		t.Errorf("Expected type 'integer', got: %s", params[0].Type)
	}
}

func TestBuildPathParamsHash(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()
	params := store.buildPathParams("/api/commits/{hash}")

	if len(params) != 1 {
		t.Fatalf("Expected 1 param, got %d", len(params))
	}
	if params[0].Name != "hash" {
		t.Errorf("Expected name 'hash', got: %s", params[0].Name)
	}
	if params[0].Type != "string" {
		t.Errorf("Expected type 'string', got: %s", params[0].Type)
	}
}

func TestBuildPathParamsMultiple(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()
	params := store.buildPathParams("/api/{uuid}/items/{id}/commits/{hash}")

	if len(params) != 3 {
		t.Fatalf("Expected 3 params, got %d", len(params))
	}

	// Check each param
	expectedNames := []string{"uuid", "id", "hash"}
	expectedTypes := []string{"uuid", "integer", "string"}
	for i, p := range params {
		if p.Name != expectedNames[i] {
			t.Errorf("Param %d: expected name '%s', got '%s'", i, expectedNames[i], p.Name)
		}
		if p.Type != expectedTypes[i] {
			t.Errorf("Param %d: expected type '%s', got '%s'", i, expectedTypes[i], p.Type)
		}
	}
}

func TestBuildPathParamsNoParams(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()
	params := store.buildPathParams("/api/users/list")

	if len(params) != 0 {
		t.Errorf("Expected 0 params for path without placeholders, got %d", len(params))
	}
}

func TestBuildPathParamsUnknownPlaceholder(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()
	params := store.buildPathParams("/api/{unknown}/items")

	// Unknown placeholders should not be recognized
	if len(params) != 0 {
		t.Errorf("Expected 0 params for unknown placeholder, got %d", len(params))
	}
}

func TestBuildPathParamsEmptyPath(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()
	params := store.buildPathParams("")

	if params != nil && len(params) != 0 {
		t.Errorf("Expected nil/empty params for empty path, got %d", len(params))
	}
}

func TestBuildPathParamsRootOnly(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()
	params := store.buildPathParams("/")

	if len(params) != 0 {
		t.Errorf("Expected 0 params for root path, got %d", len(params))
	}
}
