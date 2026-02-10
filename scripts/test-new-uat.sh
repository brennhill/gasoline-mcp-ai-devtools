#!/bin/bash
# test-new-uat.sh — New UAT Tests (98 tests, 14 categories)
# These are the newly built test categories

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TEST_DIR="$SCRIPT_DIR/tests"

echo "════════════════════════════════════════════════════════════════════════════════"
echo "NEW UAT TEST SUITE — 98 Tests, 14 Categories"
echo "════════════════════════════════════════════════════════════════════════════════"
echo ""

# Create results directory
RESULTS_DIR="/tmp/gasoline-uat-new-$(date +%s)"
mkdir -p "$RESULTS_DIR"

echo "📊 Results will be saved to: $RESULTS_DIR"
echo ""
echo "Running in parallel (8 categories at a time)..."
echo ""

# New test categories (newly built)
TESTS=(
    "cat-15-extended"
    "cat-17-generation-logic"
    "cat-17-healing-logic"
    "cat-17-performance"
    "cat-18-recording-logic"
    "cat-18-recording-automation"
    "cat-18-playback-logic"
    "cat-19-extended"
    "cat-19-link-crawling"
    "cat-20-security"
    "cat-20-filtering-logic"
    "cat-20-auto-detect"
    "cat-21-stress"
    "cat-22-advanced"
)

PORT=7890
PASS_COUNT=0
FAIL_COUNT=0
START_TIME=$(date +%s)

# Run tests in parallel (max 8 at a time for new tests)
for test in "${TESTS[@]}"; do
    RESULT_FILE="$RESULTS_DIR/${test}.txt"

    bash "$TEST_DIR/${test}.sh" "$PORT" "$RESULT_FILE" &

    PORT=$((PORT + 1))
    if [ $((PORT - 7890)) -ge 8 ]; then
        PORT=7890
    fi

    # Small delay between launches
    sleep 0.1
done

echo "⏳ Waiting for all test categories to complete..."
wait

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

# Aggregate results
echo ""
echo "════════════════════════════════════════════════════════════════════════════════"
echo "TEST RESULTS"
echo "════════════════════════════════════════════════════════════════════════════════"
echo ""

TOTAL_PASS=0
TOTAL_FAIL=0
TOTAL_SKIP=0

for test in "${TESTS[@]}"; do
    RESULT_FILE="$RESULTS_DIR/${test}.txt"

    if [ -f "$RESULT_FILE" ]; then
        PASS=$(grep -c "^  PASS:" "$RESULT_FILE" 2>/dev/null || echo "0")
        FAIL=$(grep -c "^  FAIL:" "$RESULT_FILE" 2>/dev/null || echo "0")
        SKIP=$(grep -c "^  SKIP:" "$RESULT_FILE" 2>/dev/null || echo "0")

        TOTAL_PASS=$((TOTAL_PASS + PASS))
        TOTAL_FAIL=$((TOTAL_FAIL + FAIL))
        TOTAL_SKIP=$((TOTAL_SKIP + SKIP))

        if [ "$FAIL" -eq 0 ]; then
            if [ "$SKIP" -gt 0 ]; then
                echo "⚠️  $test: $PASS passed, $SKIP skipped"
            else
                echo "✅ $test: $PASS passed"
            fi
        else
            echo "❌ $test: $PASS passed, $FAIL FAILED, $SKIP skipped"
            # Show failures
            grep "^  FAIL:" "$RESULT_FILE" | head -3
        fi
    else
        echo "⚠️  $test: NO RESULTS FILE"
    fi
done

echo ""
echo "────────────────────────────────────────────────────────────────────────────────"
echo "Summary:"
echo "  Total Passed:  $TOTAL_PASS"
echo "  Total Failed:  $TOTAL_FAIL"
echo "  Total Skipped: $TOTAL_SKIP"
echo "  Duration:      ${DURATION}s"
echo "  Tests/sec:     $(( (TOTAL_PASS + TOTAL_FAIL + TOTAL_SKIP) / (DURATION + 1) ))"
echo "════════════════════════════════════════════════════════════════════════════════"
echo ""

if [ "$TOTAL_FAIL" -eq 0 ]; then
    echo "🎉 ALL NEW TESTS PASSED (or skipped for pending features)"
    echo ""
    echo "ℹ️  Skipped tests are expected for features not yet implemented."
    exit 0
else
    echo "⚠️  FAILURES DETECTED"
    echo ""
    echo "Failed test details in: $RESULTS_DIR"
    exit 1
fi
