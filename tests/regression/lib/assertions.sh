#!/usr/bin/env bash
# Assertion functions for regression tests
# Uses jq for JSON validation

# Assert that a command succeeds
assert_success() {
    local description="$1"
    shift
    if "$@"; then
        return 0
    else
        echo "ASSERTION FAILED: $description" >&2
        echo "Command: $*" >&2
        return 1
    fi
}

# Assert that a string contains a substring
assert_contains() {
    local haystack="$1"
    local needle="$2"
    local description="${3:-String should contain '$needle'}"

    if [[ "$haystack" == *"$needle"* ]]; then
        return 0
    else
        echo "ASSERTION FAILED: $description" >&2
        echo "Expected to contain: $needle" >&2
        echo "Actual: $haystack" >&2
        return 1
    fi
}

# Assert that a string does NOT contain a substring
assert_not_contains() {
    local haystack="$1"
    local needle="$2"
    local description="${3:-String should not contain '$needle'}"

    if [[ "$haystack" != *"$needle"* ]]; then
        return 0
    else
        echo "ASSERTION FAILED: $description" >&2
        echo "Expected NOT to contain: $needle" >&2
        echo "Actual: $haystack" >&2
        return 1
    fi
}

# Assert that two strings are equal
assert_equals() {
    local expected="$1"
    local actual="$2"
    local description="${3:-Values should be equal}"

    if [[ "$expected" == "$actual" ]]; then
        return 0
    else
        echo "ASSERTION FAILED: $description" >&2
        echo "Expected: $expected" >&2
        echo "Actual: $actual" >&2
        return 1
    fi
}

# Assert that a value is not empty
assert_not_empty() {
    local value="$1"
    local description="${2:-Value should not be empty}"

    if [[ -n "$value" ]]; then
        return 0
    else
        echo "ASSERTION FAILED: $description" >&2
        return 1
    fi
}

# Assert that JSON is valid
assert_valid_json() {
    local json="$1"
    local description="${2:-Should be valid JSON}"

    if echo "$json" | jq . >/dev/null 2>&1; then
        return 0
    else
        echo "ASSERTION FAILED: $description" >&2
        echo "Invalid JSON: $json" >&2
        return 1
    fi
}

# Assert that JSON has a specific key
assert_json_has_key() {
    local json="$1"
    local key="$2"
    local description="${3:-JSON should have key '$key'}"

    if echo "$json" | jq -e "has(\"$key\")" >/dev/null 2>&1; then
        return 0
    else
        echo "ASSERTION FAILED: $description" >&2
        echo "Missing key: $key" >&2
        echo "JSON: $json" >&2
        return 1
    fi
}

# Assert that JSON path exists and is not null
assert_json_path() {
    local json="$1"
    local path="$2"
    local description="${3:-JSON path '$path' should exist}"

    if echo "$json" | jq -e "$path" >/dev/null 2>&1; then
        return 0
    else
        echo "ASSERTION FAILED: $description" >&2
        echo "Path not found: $path" >&2
        echo "JSON: $json" >&2
        return 1
    fi
}

# Assert that JSON path equals a value
assert_json_equals() {
    local json="$1"
    local path="$2"
    local expected="$3"
    local description="${4:-JSON path '$path' should equal '$expected'}"

    local actual
    actual=$(echo "$json" | jq -r "$path" 2>/dev/null)

    if [[ "$actual" == "$expected" ]]; then
        return 0
    else
        echo "ASSERTION FAILED: $description" >&2
        echo "Path: $path" >&2
        echo "Expected: $expected" >&2
        echo "Actual: $actual" >&2
        return 1
    fi
}

# Assert that JSON array is not empty
assert_json_array_not_empty() {
    local json="$1"
    local path="$2"
    local description="${3:-JSON array at '$path' should not be empty}"

    local length
    length=$(echo "$json" | jq "$path | length" 2>/dev/null)

    if [[ "$length" -gt 0 ]]; then
        return 0
    else
        echo "ASSERTION FAILED: $description" >&2
        echo "Array is empty at path: $path" >&2
        return 1
    fi
}

# Assert that JSON array has specific length
assert_json_array_length() {
    local json="$1"
    local path="$2"
    local expected_length="$3"
    local description="${4:-JSON array at '$path' should have length $expected_length}"

    local actual_length
    actual_length=$(echo "$json" | jq "$path | length" 2>/dev/null)

    if [[ "$actual_length" == "$expected_length" ]]; then
        return 0
    else
        echo "ASSERTION FAILED: $description" >&2
        echo "Expected length: $expected_length" >&2
        echo "Actual length: $actual_length" >&2
        return 1
    fi
}

# Assert MCP response is successful (has result, no error)
assert_mcp_success() {
    local response="$1"
    local description="${2:-MCP response should be successful}"

    if ! assert_valid_json "$response" "Response should be valid JSON"; then
        return 1
    fi

    if echo "$response" | jq -e '.error' >/dev/null 2>&1; then
        local error
        error=$(echo "$response" | jq -r '.error.message // .error')
        echo "ASSERTION FAILED: $description" >&2
        echo "MCP returned error: $error" >&2
        return 1
    fi

    if ! echo "$response" | jq -e '.result' >/dev/null 2>&1; then
        echo "ASSERTION FAILED: $description" >&2
        echo "MCP response missing 'result' field" >&2
        echo "Response: $response" >&2
        return 1
    fi

    return 0
}

# Assert MCP response has error
assert_mcp_error() {
    local response="$1"
    local description="${2:-MCP response should be an error}"

    if ! assert_valid_json "$response" "Response should be valid JSON"; then
        return 1
    fi

    if ! echo "$response" | jq -e '.error' >/dev/null 2>&1; then
        echo "ASSERTION FAILED: $description" >&2
        echo "Expected error but got success" >&2
        echo "Response: $response" >&2
        return 1
    fi

    return 0
}

# Assert HTTP status code
assert_http_status() {
    local url="$1"
    local expected_status="$2"
    local description="${3:-HTTP status should be $expected_status}"

    local actual_status
    actual_status=$(curl -s -o /dev/null -w "%{http_code}" "$url")

    if [[ "$actual_status" == "$expected_status" ]]; then
        return 0
    else
        echo "ASSERTION FAILED: $description" >&2
        echo "URL: $url" >&2
        echo "Expected status: $expected_status" >&2
        echo "Actual status: $actual_status" >&2
        return 1
    fi
}

# Compare JSON structure against a snapshot file (keys only, not values)
assert_json_structure() {
    local json="$1"
    local snapshot_file="$2"
    local description="${3:-JSON structure should match snapshot}"

    if [[ ! -f "$snapshot_file" ]]; then
        echo "ASSERTION FAILED: $description" >&2
        echo "Snapshot file not found: $snapshot_file" >&2
        return 1
    fi

    local actual_keys expected_keys
    actual_keys=$(echo "$json" | jq -S 'keys' 2>/dev/null)
    expected_keys=$(cat "$snapshot_file")

    if [[ "$actual_keys" == "$expected_keys" ]]; then
        return 0
    else
        echo "ASSERTION FAILED: $description" >&2
        echo "Expected keys: $expected_keys" >&2
        echo "Actual keys: $actual_keys" >&2
        return 1
    fi
}
