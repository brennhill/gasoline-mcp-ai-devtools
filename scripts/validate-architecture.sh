#!/bin/bash
# validate-architecture.sh — Enforce async queue-and-poll architecture
# Run in CI to catch architecture violations before merge
set -euo pipefail

echo "🏗️  Validating Gasoline architecture..."
echo ""

ERRORS=0

# ============================================
# 1. Critical Files Existence
# ============================================

echo "1️⃣  Checking critical files..."

CRITICAL_FILES=(
    "internal/capture/queries.go"
    "internal/capture/handlers.go"
    "internal/capture/types.go"
    "internal/queries/types.go"
    "cmd/dev-console/tools_core.go"
    "cmd/dev-console/tools_observe.go"
    "cmd/dev-console/tools_interact.go"
    "cmd/dev-console/bridge.go"
)

for file in "${CRITICAL_FILES[@]}"; do
    if [ ! -f "$file" ]; then
        echo "   ❌ MISSING: $file"
        ERRORS=$((ERRORS + 1))
    else
        echo "   ✅ $file"
    fi
done

# ============================================
# 2. Required Methods Existence
# ============================================

echo ""
echo "2️⃣  Checking required methods in queries.go..."

REQUIRED_METHODS=(
    "CreatePendingQuery"
    "CreatePendingQueryWithTimeout"
    "GetPendingQueries"
    "GetPendingQueriesForClient"
    "SetQueryResult"
    "SetQueryResultWithClient"
    "GetQueryResult"
    "RegisterCommand"
    "CompleteCommand"
    "ExpireCommand"
    "GetCommandResult"
    "GetPendingCommands"
    "GetCompletedCommands"
    "GetFailedCommands"
)

for method in "${REQUIRED_METHODS[@]}"; do
    if ! grep -q "func.*$method" internal/capture/queries.go; then
        echo "   ❌ MISSING METHOD: $method"
        ERRORS=$((ERRORS + 1))
    else
        echo "   ✅ $method"
    fi
done

# ============================================
# 3. Handler Endpoints
# ============================================

echo ""
echo "3️⃣  Checking HTTP handler endpoints..."

REQUIRED_HANDLERS=(
    "HandlePendingQueries"
    "HandleDOMResult"
    "HandleExecuteResult"
    "HandlePilotStatus"
)

for handler in "${REQUIRED_HANDLERS[@]}"; do
    if ! grep -q "func.*$handler" internal/capture/handlers.go; then
        echo "   ❌ MISSING HANDLER: $handler"
        ERRORS=$((ERRORS + 1))
    else
        echo "   ✅ $handler"
    fi
done

# ============================================
# 4. MCP Tool Handlers
# ============================================

echo ""
echo "4️⃣  Checking MCP tool handlers..."

MCP_TOOL_HANDLERS=(
    "toolObserveCommandResult"
    "toolObservePendingCommands"
    "toolObserveFailedCommands"
    "handlePilotExecuteJS"
    "handleBrowserActionNavigate"
)

for handler in "${MCP_TOOL_HANDLERS[@]}"; do
    if ! grep -rq "func.*$handler" cmd/dev-console/tools_*.go; then
        echo "   ❌ MISSING TOOL HANDLER: $handler"
        ERRORS=$((ERRORS + 1))
    else
        echo "   ✅ $handler"
    fi
done

# ============================================
# 5. No Stub Implementations
# ============================================

echo ""
echo "5️⃣  Checking for stub implementations..."

# Check handlers.go for stub returns
if grep -q 'queries.*\[\]interface{}{}' internal/capture/handlers.go; then
    echo "   ❌ STUB DETECTED: handlers.go returns empty array"
    ERRORS=$((ERRORS + 1))
else
    echo "   ✅ No stub in handlers.go"
fi

# Check tools_observe.go for stub returns in command result observer
if grep -rq 'func (h \*ToolHandler) toolObserveCommandResult.*{' cmd/dev-console/tools_*.go; then
    # Extract function body and check if it calls GetCommandResult
    if grep -rA 20 'func (h \*ToolHandler) toolObserveCommandResult' cmd/dev-console/tools_*.go | grep -q 'GetCommandResult'; then
        echo "   ✅ toolObserveCommandResult calls GetCommandResult"
    else
        echo "   ❌ STUB DETECTED: toolObserveCommandResult doesn't call GetCommandResult"
        ERRORS=$((ERRORS + 1))
    fi
fi

# ============================================
# 6. Integration Test Exists
# ============================================

echo ""
echo "6️⃣  Checking integration tests..."

if [ ! -f "internal/capture/async_queue_integration_test.go" ]; then
    echo "   ❌ MISSING: async_queue_integration_test.go"
    ERRORS=$((ERRORS + 1))
else
    echo "   ✅ async_queue_integration_test.go exists"
fi

# ============================================
# 7. Run Integration Tests
# ============================================

echo ""
echo "7️⃣  Running integration tests..."

if go test -v ./internal/capture -run TestAsyncQueueIntegration > /tmp/gasoline-integration-test.log 2>&1; then
    echo "   ✅ Integration tests pass"
else
    echo "   ❌ Integration tests FAILED"
    echo ""
    echo "   Test output:"
    tail -30 /tmp/gasoline-integration-test.log
    ERRORS=$((ERRORS + 1))
fi

# ============================================
# 8. Constants Check
# ============================================

echo ""
echo "8️⃣  Checking critical constants..."

if ! grep -q 'AsyncCommandTimeout.*30.*time.Second' internal/queries/types.go; then
    echo "   ❌ AsyncCommandTimeout not set to 30s"
    ERRORS=$((ERRORS + 1))
else
    echo "   ✅ AsyncCommandTimeout = 30s"
fi

if ! grep -q 'maxPendingQueries.*=.*5' internal/capture/types.go; then
    echo "   ⚠️  WARNING: maxPendingQueries not found or not set to 5"
else
    echo "   ✅ maxPendingQueries = 5"
fi

# ============================================
# 9. Documentation Check
# ============================================

echo ""
echo "9️⃣  Checking documentation..."

if [ ! -f "docs/async-queue-correlation-tracking.md" ]; then
    echo "   ⚠️  WARNING: Missing async-queue-correlation-tracking.md"
else
    echo "   ✅ async-queue-correlation-tracking.md exists"
fi

# ============================================
# Summary
# ============================================

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ $ERRORS -eq 0 ]; then
    echo "✅ Architecture validation PASSED"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    exit 0
else
    echo "❌ Architecture validation FAILED with $ERRORS error(s)"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "The async queue-and-poll architecture is broken."
    echo "DO NOT merge this change."
    echo ""
    echo "See: docs/async-queue-correlation-tracking.md"
    echo "Or ask: 'How do I restore the async queue implementation?'"
    exit 1
fi
