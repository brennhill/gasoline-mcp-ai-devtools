#!/bin/bash
# test-original-uat.sh — Original/Proven UAT Tests (54 tests, 20 categories)
# These are the stable, battle-tested test categories

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TEST_DIR="$SCRIPT_DIR/tests"

echo "════════════════════════════════════════════════════════════════════════════════"
echo "ORIGINAL UAT TEST SUITE — 54 Tests, 20 Categories"
echo "════════════════════════════════════════════════════════════════════════════════"
echo ""

# Create results directory
RESULTS_DIR="/tmp/gasoline-uat-original-$(date +%s)"
mkdir -p "$RESULTS_DIR"

echo "📊 Results will be saved to: $RESULTS_DIR"
echo ""
echo "Running in parallel (16 categories at once)..."
echo ""

# Original test categories (proven stable)
TESTS=(
    "cat-01-protocol"
    "cat-02-observe"
    "cat-03-generate"
    "cat-04-configure"
    "cat-05-interact"
    "cat-06-lifecycle"
    "cat-07-concurrency"
    "cat-08-security"
    "cat-09-http"
    "cat-10-regression"
    "cat-11-data-pipeline"
    "cat-12-rich-actions"
    "cat-13-pilot-contract"
    "cat-14-extension-startup"
    "cat-15-pilot-success-path"
    "cat-16-api-contract"
    "cat-17-reproduction"
    "cat-18-recording"
    "cat-19-link-health"
    "cat-20-noise-persistence"
)

PORT=7890
PASS_COUNT=0
FAIL_COUNT=0
START_TIME=$(date +%s)

# Run tests in parallel (max 16 at a time)
for test in "${TESTS[@]}"; do
    RESULT_FILE="$RESULTS_DIR/${test}.txt"

    bash "$TEST_DIR/${test}.sh" "$PORT" "$RESULT_FILE" &

    PORT=$((PORT + 1))
    if [ $((PORT - 7890)) -ge 16 ]; then
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

for test in "${TESTS[@]}"; do
    RESULT_FILE="$RESULTS_DIR/${test}.txt"

    if [ -f "$RESULT_FILE" ]; then
        PASS=$(grep -c "^  PASS:" "$RESULT_FILE" 2>/dev/null || echo "0")
        FAIL=$(grep -c "^  FAIL:" "$RESULT_FILE" 2>/dev/null || echo "0")

        TOTAL_PASS=$((TOTAL_PASS + PASS))
        TOTAL_FAIL=$((TOTAL_FAIL + FAIL))

        if [ "$FAIL" -eq 0 ]; then
            echo "✅ $test: $PASS passed"
        else
            echo "❌ $test: $PASS passed, $FAIL FAILED"
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
echo "  Duration:      ${DURATION}s"
echo "  Tests/sec:     $(( (TOTAL_PASS + TOTAL_FAIL) / (DURATION + 1) ))"
echo "════════════════════════════════════════════════════════════════════════════════"
echo ""

if [ "$TOTAL_FAIL" -eq 0 ]; then
    echo "🎉 ALL ORIGINAL TESTS PASSED"
    exit 0
else
    echo "⚠️  FAILURES DETECTED"
    echo ""
    echo "Failed test details in: $RESULTS_DIR"
    exit 1
fi
