#!/bin/bash
# test-all-tools-comprehensive.sh — Parallel UAT runner for Gasoline MCP.
# Launches 8 groups of category tests, collects results, prints summary.
# Compatible with bash 3.2+ (macOS default).
# NO set -e: we need to collect all results even if some groups fail.

# ── Dependency Checks ─────────────────────────────────────
check_deps() {
    local missing=""

    for cmd in jq curl lsof; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            missing="$missing $cmd"
        fi
    done

    # timeout may be gtimeout on macOS
    if ! command -v timeout >/dev/null 2>&1 && ! command -v gtimeout >/dev/null 2>&1; then
        missing="$missing timeout(brew install coreutils)"
    fi

    if [ -n "$missing" ]; then
        echo "FATAL: Missing dependencies:$missing" >&2
        exit 1
    fi
}

check_deps

# ── Timeout Compatibility ─────────────────────────────────
if command -v timeout >/dev/null 2>&1; then
    TIMEOUT_CMD="timeout"
elif command -v gtimeout >/dev/null 2>&1; then
    TIMEOUT_CMD="gtimeout"
else
    echo "FATAL: 'timeout' not found. Install with: brew install coreutils" >&2
    exit 1
fi

# ── Resolve Binary ────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TESTS_DIR="$SCRIPT_DIR/tests"

if [ -x "$PROJECT_ROOT/gasoline-mcp" ]; then
    WRAPPER="$PROJECT_ROOT/gasoline-mcp"
elif command -v gasoline-mcp >/dev/null 2>&1; then
    WRAPPER="$(command -v gasoline-mcp)"
else
    echo "FATAL: gasoline-mcp not found in $PROJECT_ROOT or PATH" >&2
    exit 1
fi

# ── Temp Dir for Results ──────────────────────────────────
RESULTS_DIR="$(mktemp -d)"
OVERALL_START="$(date +%s)"

echo ""
echo "############################################################"
echo "# GASOLINE MCP — COMPREHENSIVE UAT"
echo "############################################################"
echo ""
echo "Binary:     $WRAPPER"
echo "Tests dir:  $TESTS_DIR"
echo "Results:    $RESULTS_DIR"
echo ""

# ── Port Assignments ──────────────────────────────────────
# Each parallel group gets its own port so it can spin up an
# independent daemon. This lets all groups run simultaneously
# without contention — total UAT wall time is ~the slowest group
# instead of the sum of all groups.
PORT_GROUP1=7890  # cat-01-protocol
PORT_GROUP2=7891  # cat-02-observe
PORT_GROUP3=7892  # cat-03-generate
PORT_GROUP4=7893  # cat-04-configure + cat-05-interact (sequential)
PORT_GROUP5=7894  # cat-07-concurrency
PORT_GROUP6=7895  # cat-08-security + cat-09-http (sequential)
PORT_GROUP7=7896  # cat-06-lifecycle
PORT_GROUP8=7897  # cat-10-regression
PORT_GROUP9=7898  # cat-11-data-pipeline
PORT_GROUP10=7899 # cat-12-rich-actions
PORT_GROUP11=7900 # cat-13-pilot-contract
PORT_GROUP12=7901 # cat-14-extension-startup
PORT_GROUP13=7902 # cat-15-pilot-success-path
PORT_GROUP14=7903 # cat-16-api-contract
PORT_GROUP15=7904 # cat-18-recording
PORT_GROUP16=7905 # cat-19-link-health
PORT_GROUP18=7907 # cat-23-draw-mode
PORT_GROUP19=7908 # cat-24-upload
PORT_GROUP20=7909 # cat-25-annotations

# Kill anything on our ports before starting
for port in $PORT_GROUP1 $PORT_GROUP2 $PORT_GROUP3 $PORT_GROUP4 $PORT_GROUP5 $PORT_GROUP6 $PORT_GROUP7 $PORT_GROUP8 $PORT_GROUP9 $PORT_GROUP10 $PORT_GROUP11 $PORT_GROUP12 $PORT_GROUP13 $PORT_GROUP14 $PORT_GROUP15 $PORT_GROUP16 $PORT_GROUP18 $PORT_GROUP19 $PORT_GROUP20; do
    lsof -ti :"$port" 2>/dev/null | xargs kill -9 2>/dev/null || true
done
sleep 0.5

# ── Launch Groups ─────────────────────────────────────────
PIDS=""

# Group 1: Protocol (single script)
(
    cd "$PROJECT_ROOT" || exit
    "$TIMEOUT_CMD" 120 bash "$TESTS_DIR/cat-01-protocol.sh" "$PORT_GROUP1" "$RESULTS_DIR/results-01.txt" \
        > "$RESULTS_DIR/output-01.txt" 2>&1
) &
PIDS="$PIDS $!"

# Group 2: Observe (single script)
(
    cd "$PROJECT_ROOT" || exit
    "$TIMEOUT_CMD" 120 bash "$TESTS_DIR/cat-02-observe.sh" "$PORT_GROUP2" "$RESULTS_DIR/results-02.txt" \
        > "$RESULTS_DIR/output-02.txt" 2>&1
) &
PIDS="$PIDS $!"

# Group 3: Generate (single script)
(
    cd "$PROJECT_ROOT" || exit
    "$TIMEOUT_CMD" 120 bash "$TESTS_DIR/cat-03-generate.sh" "$PORT_GROUP3" "$RESULTS_DIR/results-03.txt" \
        > "$RESULTS_DIR/output-03.txt" 2>&1
) &
PIDS="$PIDS $!"

# Group 4: Configure then Interact (sequential, same port)
(
    cd "$PROJECT_ROOT" || exit
    "$TIMEOUT_CMD" 120 bash "$TESTS_DIR/cat-04-configure.sh" "$PORT_GROUP4" "$RESULTS_DIR/results-04.txt" \
        > "$RESULTS_DIR/output-04.txt" 2>&1
    "$TIMEOUT_CMD" 120 bash "$TESTS_DIR/cat-05-interact.sh" "$PORT_GROUP4" "$RESULTS_DIR/results-05.txt" \
        > "$RESULTS_DIR/output-05.txt" 2>&1
) &
PIDS="$PIDS $!"

# Group 5: Concurrency (single script)
(
    cd "$PROJECT_ROOT" || exit
    "$TIMEOUT_CMD" 120 bash "$TESTS_DIR/cat-07-concurrency.sh" "$PORT_GROUP5" "$RESULTS_DIR/results-07.txt" \
        > "$RESULTS_DIR/output-07.txt" 2>&1
) &
PIDS="$PIDS $!"

# Group 6: Security then HTTP (sequential, same port)
(
    cd "$PROJECT_ROOT" || exit
    "$TIMEOUT_CMD" 120 bash "$TESTS_DIR/cat-08-security.sh" "$PORT_GROUP6" "$RESULTS_DIR/results-08.txt" \
        > "$RESULTS_DIR/output-08.txt" 2>&1
    "$TIMEOUT_CMD" 120 bash "$TESTS_DIR/cat-09-http.sh" "$PORT_GROUP6" "$RESULTS_DIR/results-09.txt" \
        > "$RESULTS_DIR/output-09.txt" 2>&1
) &
PIDS="$PIDS $!"

# Group 7: Lifecycle (single script)
(
    cd "$PROJECT_ROOT" || exit
    "$TIMEOUT_CMD" 120 bash "$TESTS_DIR/cat-06-lifecycle.sh" "$PORT_GROUP7" "$RESULTS_DIR/results-06.txt" \
        > "$RESULTS_DIR/output-06.txt" 2>&1
) &
PIDS="$PIDS $!"

# Group 8: Regression (single script)
(
    cd "$PROJECT_ROOT" || exit
    "$TIMEOUT_CMD" 120 bash "$TESTS_DIR/cat-10-regression.sh" "$PORT_GROUP8" "$RESULTS_DIR/results-10.txt" \
        > "$RESULTS_DIR/output-10.txt" 2>&1
) &
PIDS="$PIDS $!"

# Group 9: Data Pipeline (single script)
(
    cd "$PROJECT_ROOT" || exit
    "$TIMEOUT_CMD" 120 bash "$TESTS_DIR/cat-11-data-pipeline.sh" "$PORT_GROUP9" "$RESULTS_DIR/results-11.txt" \
        > "$RESULTS_DIR/output-11.txt" 2>&1
) &
PIDS="$PIDS $!"

# Group 10: Rich Action Results (single script)
(
    cd "$PROJECT_ROOT" || exit
    "$TIMEOUT_CMD" 120 bash "$TESTS_DIR/cat-12-rich-actions.sh" "$PORT_GROUP10" "$RESULTS_DIR/results-12.txt" \
        > "$RESULTS_DIR/output-12.txt" 2>&1
) &
PIDS="$PIDS $!"

# Group 11: Pilot Contract Tests (single script)
(
    cd "$PROJECT_ROOT" || exit
    "$TIMEOUT_CMD" 120 bash "$TESTS_DIR/cat-13-pilot-contract.sh" "$PORT_GROUP11" "$RESULTS_DIR/results-13.txt" \
        > "$RESULTS_DIR/output-13.txt" 2>&1
) &
PIDS="$PIDS $!"

# Group 12: Extension Startup (single script)
(
    cd "$PROJECT_ROOT" || exit
    "$TIMEOUT_CMD" 120 bash "$TESTS_DIR/cat-14-extension-startup.sh" "$PORT_GROUP12" "$RESULTS_DIR/results-14.txt" \
        > "$RESULTS_DIR/output-14.txt" 2>&1
) &
PIDS="$PIDS $!"

# Group 13: Pilot Success Path (single script)
(
    cd "$PROJECT_ROOT" || exit
    "$TIMEOUT_CMD" 120 bash "$TESTS_DIR/cat-15-pilot-success-path.sh" "$PORT_GROUP13" "$RESULTS_DIR/results-15.txt" \
        > "$RESULTS_DIR/output-15.txt" 2>&1
) &
PIDS="$PIDS $!"

# Group 14: API Contract (single script)
(
    cd "$PROJECT_ROOT" || exit
    "$TIMEOUT_CMD" 120 bash "$TESTS_DIR/cat-16-api-contract.sh" "$PORT_GROUP14" "$RESULTS_DIR/results-16.txt" \
        > "$RESULTS_DIR/output-16.txt" 2>&1
) &
PIDS="$PIDS $!"

# Group 15: Recording & Audio (single script)
(
    cd "$PROJECT_ROOT" || exit
    "$TIMEOUT_CMD" 120 bash "$TESTS_DIR/cat-18-recording.sh" "$PORT_GROUP15" "$RESULTS_DIR/results-18.txt" \
        > "$RESULTS_DIR/output-18.txt" 2>&1
) &
PIDS="$PIDS $!"

# Group 16: Link Health Analyzer (single script)
(
    cd "$PROJECT_ROOT" || exit
    bash "$TESTS_DIR/cat-19-link-health.sh" "$PORT_GROUP16" "$RESULTS_DIR/results-19.txt" \
        > "$RESULTS_DIR/output-19.txt" 2>&1
) &
PIDS="$PIDS $!"

# Group 17: Noise Rule Persistence (single script)
PORT_GROUP17=7906  # cat-20-noise-persistence
(
    cd "$PROJECT_ROOT" || exit
    bash "$TESTS_DIR/cat-20-noise-persistence.sh" "$PORT_GROUP17" "$RESULTS_DIR/results-20.txt" \
        > "$RESULTS_DIR/output-20.txt" 2>&1
) &
PIDS="$PIDS $!"

# Group 18: Draw Mode (single script)
(
    cd "$PROJECT_ROOT" || exit
    "$TIMEOUT_CMD" 120 bash "$TESTS_DIR/cat-23-draw-mode.sh" "$PORT_GROUP18" "$RESULTS_DIR/results-23.txt" \
        > "$RESULTS_DIR/output-23.txt" 2>&1
) &
PIDS="$PIDS $!"

# Group 19: Upload (single script)
(
    cd "$PROJECT_ROOT" || exit
    "$TIMEOUT_CMD" 120 bash "$TESTS_DIR/cat-24-upload.sh" "$PORT_GROUP19" "$RESULTS_DIR/results-24.txt" \
        > "$RESULTS_DIR/output-24.txt" 2>&1
) &
PIDS="$PIDS $!"

# Group 20: Annotation Integration (single script)
(
    cd "$PROJECT_ROOT" || exit
    "$TIMEOUT_CMD" 120 bash "$TESTS_DIR/cat-25-annotations.sh" "$PORT_GROUP20" "$RESULTS_DIR/results-25.txt" \
        > "$RESULTS_DIR/output-25.txt" 2>&1
) &
PIDS="$PIDS $!"

# ── Wait for All Groups ──────────────────────────────────
echo "Running 20 parallel groups..."
echo ""

# Master watchdog: kill all groups if UAT exceeds 5 minutes total
WATCHDOG_TIMEOUT=300
(
    sleep "$WATCHDOG_TIMEOUT"
    echo ""
    echo "WATCHDOG: UAT exceeded ${WATCHDOG_TIMEOUT}s master timeout. Killing all groups." >&2
    for pid in $PIDS; do
        kill "$pid" 2>/dev/null || true
    done
    # Also kill any daemons on our ports
    for port in $PORT_GROUP1 $PORT_GROUP2 $PORT_GROUP3 $PORT_GROUP4 $PORT_GROUP5 $PORT_GROUP6 $PORT_GROUP7 $PORT_GROUP8 $PORT_GROUP9 $PORT_GROUP10 $PORT_GROUP11 $PORT_GROUP12 $PORT_GROUP13 $PORT_GROUP14 $PORT_GROUP15 $PORT_GROUP16 $PORT_GROUP18 $PORT_GROUP19 $PORT_GROUP20; do
        lsof -ti :"$port" 2>/dev/null | xargs kill -9 2>/dev/null || true
    done
) &
WATCHDOG_PID="$!"

for pid in $PIDS; do
    wait "$pid" 2>/dev/null || true
done

# Cancel the watchdog — all groups finished in time
kill "$WATCHDOG_PID" 2>/dev/null || true
wait "$WATCHDOG_PID" 2>/dev/null || true

# ── Collect and Display Results ───────────────────────────

# Category display order and default names
CAT_IDS="01 02 03 04 05 06 07 08 09 10 11 12 13 14 15 16 18 19 20 23 24 25"
get_default_name() {
    case "$1" in
        01) echo "Protocol Compliance" ;;
        02) echo "Observe Tool" ;;
        03) echo "Generate Tool" ;;
        04) echo "Configure Tool" ;;
        05) echo "Interact Tool" ;;
        06) echo "Server Lifecycle" ;;
        07) echo "Concurrency" ;;
        08) echo "Security" ;;
        09) echo "HTTP Endpoints" ;;
        10) echo "Regression Guards" ;;
        11) echo "Data Pipeline" ;;
        12) echo "Rich Action Results" ;;
        13) echo "Pilot State Contract" ;;
        14) echo "Extension Startup" ;;
        15) echo "Pilot Success Path" ;;
        16) echo "API Contract" ;;
        18) echo "Recording & Audio" ;;
        19) echo "Link Health Analyzer" ;;
        20) echo "Noise Persistence" ;;
        23) echo "Draw Mode" ;;
        24) echo "File Upload" ;;
        25) echo "Annotation Integration" ;;
        *)  echo "Unknown" ;;
    esac
}

TOTAL_PASS=0
TOTAL_FAIL=0

# Print category outputs in order
for cat_id in $CAT_IDS; do
    output_file="$RESULTS_DIR/output-${cat_id}.txt"
    if [ -f "$output_file" ]; then
        cat "$output_file"
    fi
done

echo ""
echo ""

# ── Summary Table ─────────────────────────────────────────
echo "############################################################"
echo "# COMPREHENSIVE UAT RESULTS"
echo "############################################################"
echo ""
printf "%-28s | %4s | %4s | %5s | %5s\n" "Category" "Pass" "Fail" "Total" "Time"
echo "------------------------------------------------------------"

for cat_id in $CAT_IDS; do
    results_file="$RESULTS_DIR/results-${cat_id}.txt"
    cat_pass=0
    cat_fail=0
    cat_elapsed="?"
    cat_name="$(get_default_name "$cat_id")"

    if [ -f "$results_file" ]; then
        # Source the results file to read variables
        eval "$(grep '^PASS_COUNT=' "$results_file" 2>/dev/null)"
        eval "$(grep '^FAIL_COUNT=' "$results_file" 2>/dev/null)"
        eval "$(grep '^ELAPSED=' "$results_file" 2>/dev/null)"
        eval "$(grep '^CATEGORY_NAME=' "$results_file" 2>/dev/null)"

        cat_pass="${PASS_COUNT:-0}"
        cat_fail="${FAIL_COUNT:-0}"
        cat_elapsed="${ELAPSED:-?}"
        if [ -n "$CATEGORY_NAME" ]; then
            cat_name="$CATEGORY_NAME"
        fi

        # Reset for next iteration
        unset PASS_COUNT FAIL_COUNT ELAPSED CATEGORY_NAME
    fi

    cat_total="$((cat_pass + cat_fail))"
    TOTAL_PASS="$((TOTAL_PASS + cat_pass))"
    TOTAL_FAIL="$((TOTAL_FAIL + cat_fail))"

    printf "%2s. %-24s | %4d | %4d | %5d | %3ss\n" \
        "$cat_id" "$cat_name" "$cat_pass" "$cat_fail" "$cat_total" "$cat_elapsed"
done

TOTAL_ALL="$((TOTAL_PASS + TOTAL_FAIL))"
OVERALL_ELAPSED="$(( "$(date +%s)" - OVERALL_START ))"

echo "------------------------------------------------------------"
printf "%-28s | %4d | %4d | %5d | %3ss\n" \
    "TOTAL" "$TOTAL_PASS" "$TOTAL_FAIL" "$TOTAL_ALL" "$OVERALL_ELAPSED"

echo ""

# ── Final Verdict ─────────────────────────────────────────
if [ "$TOTAL_FAIL" -eq 0 ] && [ "$TOTAL_PASS" -gt 0 ]; then
    echo "ALL $TOTAL_PASS TESTS PASSED"
else
    echo "FAILURES: $TOTAL_FAIL of $TOTAL_ALL tests failed"
fi

echo ""

# ── Cleanup ───────────────────────────────────────────────
# Kill any remaining daemons on our ports
for port in $PORT_GROUP1 $PORT_GROUP2 $PORT_GROUP3 $PORT_GROUP4 $PORT_GROUP5 $PORT_GROUP6 $PORT_GROUP7 $PORT_GROUP8 $PORT_GROUP9 $PORT_GROUP10 $PORT_GROUP11 $PORT_GROUP12 $PORT_GROUP13 $PORT_GROUP14 $PORT_GROUP15 $PORT_GROUP16 $PORT_GROUP18 $PORT_GROUP19 $PORT_GROUP20; do
    lsof -ti :"$port" 2>/dev/null | xargs kill -9 2>/dev/null || true
done

rm -rf "$RESULTS_DIR"

# Exit code
if [ "$TOTAL_FAIL" -gt 0 ]; then
    exit 1
fi
exit 0
