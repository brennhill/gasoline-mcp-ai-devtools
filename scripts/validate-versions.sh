#!/bin/bash
# Validate that all version numbers in the project match
set -euo pipefail

VERSION=$(tr -d '[:space:]' < VERSION)
CMD_PKG="${GASOLINE_CMD_PKG:-./cmd/dev-console}"
CMD_DIR="${CMD_PKG#./}"

if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "❌ VERSION file is not strict semver: $VERSION"
    exit 1
fi

echo "Checking all version references match: $VERSION"

ERRORS=0

# Function to check version in file
check_version() {
    local file=$1
    local pattern=$2
    local found_version
    found_version=$(grep -E "$pattern" "$file" | head -1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo "NOT_FOUND")

    if [ "$found_version" != "$VERSION" ]; then
        echo "❌ $file: Expected $VERSION, found $found_version"
        ERRORS=$((ERRORS + 1))
    else
        echo "✅ $file"
    fi
}

# Check all locations
check_version "$CMD_DIR/main.go" 'var version = "'
check_version "extension/manifest.json" '"version":'
check_version "extension/package.json" '"version":'
check_version "server/package.json" '"version":'
check_version "npm/darwin-arm64/package.json" '"version":'
check_version "npm/darwin-x64/package.json" '"version":'
check_version "npm/linux-arm64/package.json" '"version":'
check_version "npm/linux-x64/package.json" '"version":'
check_version "npm/win32-x64/package.json" '"version":'
check_version "README.md" 'version-.*-green'
check_version "npm/gasoline-mcp/package.json" '"version":'
check_version "packages/gasoline-ci/package.json" '"version":'
check_version "packages/gasoline-playwright/package.json" '"version":'

# Makefile version source sanity check.
if grep -q '^VERSION :=' Makefile; then
    echo "✅ Makefile VERSION assignment exists"
else
    echo "❌ Makefile VERSION assignment missing"
    ERRORS=$((ERRORS + 1))
fi

# File-specific version strategy checks (not semver literals in source)
echo ""
echo "Checking file-specific version strategies..."

if grep -q "const VERSION = require('../package.json').version" server/scripts/install.js; then
    echo "✅ server/scripts/install.js uses package.json version source"
else
    echo "❌ server/scripts/install.js does not source version from package.json"
    ERRORS=$((ERRORS + 1))
fi

if grep -q '"version": "VERSION"' "$CMD_DIR/testdata/mcp-initialize.golden.json"; then
    echo "✅ $CMD_DIR/testdata/mcp-initialize.golden.json uses VERSION placeholder"
else
    echo "❌ $CMD_DIR/testdata/mcp-initialize.golden.json missing VERSION placeholder"
    ERRORS=$((ERRORS + 1))
fi

if grep -q 'var version = "dev"' internal/export/export_sarif.go; then
    echo "✅ internal/export/export_sarif.go uses build-time injected version fallback"
else
    echo "❌ internal/export/export_sarif.go missing build-time version fallback (var version = \"dev\")"
    ERRORS=$((ERRORS + 1))
fi

# Special check: optionalDependencies in gasoline-mcp
echo ""
echo "Checking optionalDependencies in npm/gasoline-mcp/package.json..."
DEPS=$(grep -A 5 '"optionalDependencies"' npm/gasoline-mcp/package.json | grep '@brennhill' | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | sort -u)
DEP_COUNT=$(echo "$DEPS" | wc -l | tr -d ' ')

if [ "$DEP_COUNT" != "1" ]; then
    echo "❌ optionalDependencies have multiple versions: $DEPS"
    ERRORS=$((ERRORS + 1))
else
    DEP_VERSION=$(echo "$DEPS" | tr -d '\n')
    if [ "$DEP_VERSION" != "$VERSION" ]; then
        echo "❌ optionalDependencies version $DEP_VERSION != $VERSION"
        ERRORS=$((ERRORS + 1))
    else
        echo "✅ optionalDependencies all point to $VERSION"
    fi
fi

# Check PyPI packages
echo ""
echo "Checking PyPI packages..."
PYPI_ERRORS=0
for f in pypi/*/pyproject.toml; do
    if [ -f "$f" ]; then
        PYPI_VERSION=$(grep '^version = ' "$f" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo "NOT_FOUND")
        if [ "$PYPI_VERSION" != "$VERSION" ]; then
            echo "❌ $f: Expected $VERSION, found $PYPI_VERSION"
            PYPI_ERRORS=$((PYPI_ERRORS + 1))
        else
            echo "✅ $f"
        fi
    fi
done

for f in pypi/*/*/__init__.py; do
    if [ -f "$f" ]; then
        PYPI_VERSION=$(grep '__version__ = ' "$f" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo "NOT_FOUND")
        if [ "$PYPI_VERSION" != "$VERSION" ] && [ "$PYPI_VERSION" != "NOT_FOUND" ]; then
            echo "❌ $f: Expected $VERSION, found $PYPI_VERSION"
            PYPI_ERRORS=$((PYPI_ERRORS + 1))
        else
            echo "✅ $f"
        fi
    fi
done

if [ $PYPI_ERRORS -gt 0 ]; then
    ERRORS=$((ERRORS + PYPI_ERRORS))
fi

echo ""
if [ $ERRORS -eq 0 ]; then
    echo "✅ All version numbers match: $VERSION"
    exit 0
else
    echo "❌ Found $ERRORS version mismatches"
    exit 1
fi
