#!/bin/bash
# Validate that all version numbers in the project match
set -euo pipefail

VERSION=$(grep "^VERSION :=" Makefile | awk '{print $3}')

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
check_version "Makefile" "^VERSION :="
check_version "cmd/dev-console/main.go" 'var version = "'
check_version "extension/manifest.json" '"version":'
check_version "extension/package.json" '"version":'
check_version "server/package.json" '"version":'
check_version "server/scripts/install.js" "const VERSION ="
check_version "npm/darwin-arm64/package.json" '"version":'
check_version "npm/darwin-x64/package.json" '"version":'
check_version "npm/linux-arm64/package.json" '"version":'
check_version "npm/linux-x64/package.json" '"version":'
check_version "npm/win32-x64/package.json" '"version":'
check_version "README.md" 'version-.*-green'
check_version "cmd/dev-console/testdata/mcp-initialize.golden.json" '"version":'
check_version "npm/gasoline-mcp/package.json" '"version":'
check_version "packages/gasoline-ci/package.json" '"version":'
check_version "packages/gasoline-playwright/package.json" '"version":'
check_version "internal/export/export_sarif.go" 'const version = "'

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
