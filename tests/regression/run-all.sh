#!/usr/bin/env bash
# External Regression Test Suite for Gasoline MCP Server
#
# This test suite is intentionally external to the main codebase
# to catch regressions that internal tests might miss.
#
# Usage:
#   ./run-all.sh                    # Run all tests
#   ./run-all.sh 01-startup         # Run only startup tests
#   ./run-all.sh --update-snapshots # Update snapshot files

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Source common functions
source lib/common.sh
source lib/assertions.sh

# Parse arguments
UPDATE_SNAPSHOTS=false
TEST_FILTER="${1:-}"

if [[ "$TEST_FILTER" == "--update-snapshots" ]]; then
    UPDATE_SNAPSHOTS=true
    TEST_FILTER=""
fi

# Counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

# Run a single test file
run_test_file() {
    local test_file="$1"
    local test_name
    test_name=$(basename "$test_file" .sh)

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Running: $test_file"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    if bash "$test_file"; then
        ((TESTS_PASSED++))
        log_pass "$test_file"
        return 0
    else
        ((TESTS_FAILED++))
        log_fail "$test_file"
        return 1
    fi
}

# Find and run test files
run_tests() {
    local filter="${1:-}"
    local test_dirs=()

    # Find test directories in order
    for dir in "$SCRIPT_DIR"/[0-9][0-9]-*/; do
        if [[ -d "$dir" ]]; then
            local dir_name
            dir_name=$(basename "$dir")

            # Apply filter if specified
            if [[ -n "$filter" && "$dir_name" != *"$filter"* ]]; then
                continue
            fi

            test_dirs+=("$dir")
        fi
    done

    if [[ ${#test_dirs[@]} -eq 0 ]]; then
        echo "No test directories found matching filter: $filter"
        return 1
    fi

    # Run each test directory
    for dir in "${test_dirs[@]}"; do
        local dir_name
        dir_name=$(basename "$dir")

        echo ""
        echo "╔══════════════════════════════════════════╗"
        echo "║ Test Suite: $dir_name"
        echo "╚══════════════════════════════════════════╝"

        # Find test files in directory
        for test_file in "$dir"/test-*.sh; do
            if [[ -f "$test_file" ]]; then
                run_test_file "$test_file" || true
            fi
        done
    done
}

# Print summary
print_summary() {
    local total=$((TESTS_PASSED + TESTS_FAILED + TESTS_SKIPPED))

    echo ""
    echo "══════════════════════════════════════════"
    echo "Test Summary"
    echo "══════════════════════════════════════════"
    echo -e "${GREEN}Passed:${NC}  $TESTS_PASSED"
    echo -e "${RED}Failed:${NC}  $TESTS_FAILED"
    echo -e "${YELLOW}Skipped:${NC} $TESTS_SKIPPED"
    echo "Total:   $total"
    echo "══════════════════════════════════════════"

    if [[ $TESTS_FAILED -gt 0 ]]; then
        echo -e "${RED}REGRESSION TESTS FAILED${NC}"
        return 1
    else
        echo -e "${GREEN}ALL TESTS PASSED${NC}"
        return 0
    fi
}

# Main
main() {
    echo "╔══════════════════════════════════════════╗"
    echo "║   Gasoline External Regression Tests     ║"
    echo "╚══════════════════════════════════════════╝"
    echo ""
    echo "Binary: $GASOLINE_BINARY"
    echo "Port:   $GASOLINE_PORT"
    echo "Filter: ${TEST_FILTER:-<all>}"
    echo ""

    # Check prerequisites
    if ! command -v jq &>/dev/null; then
        echo "ERROR: jq is required but not installed"
        exit 1
    fi

    if ! command -v curl &>/dev/null; then
        echo "ERROR: curl is required but not installed"
        exit 1
    fi

    # Check binary exists
    if [[ ! -x "$GASOLINE_BINARY" ]]; then
        # Try to find it relative to repo root
        local repo_root
        repo_root=$(cd "$SCRIPT_DIR/../.." && pwd)

        if [[ -x "$repo_root/dist/gasoline" ]]; then
            export GASOLINE_BINARY="$repo_root/dist/gasoline"
        else
            echo "ERROR: Gasoline binary not found"
            echo "Expected: $GASOLINE_BINARY"
            echo "Run: go build -o dist/gasoline ./cmd/dev-console"
            exit 1
        fi
    fi

    # Find a free port
    GASOLINE_PORT=$(find_free_port)
    export GASOLINE_PORT
    export GASOLINE_URL="http://127.0.0.1:${GASOLINE_PORT}"

    # Run tests
    run_tests "$TEST_FILTER"

    # Print summary
    print_summary
}

main "$@"
