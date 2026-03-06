#!/bin/bash
# check-import-boundaries.sh — Enforce module boundary rules for TypeScript.
#
# Rules:
#   - content/ must NOT import from background/
#   - inject/ must NOT import from background/
#   - inject/ must NOT import from content/
#   - lib/ must NOT import from background/ or content/
#
# Exit code: 0 if all boundaries hold, 1 if violations found.

set -euo pipefail

echo "=== Checking TypeScript import boundaries ==="

VIOLATIONS=0

check_boundary() {
    local from_dir="$1"
    local forbidden_dir="$2"

    # Look for relative imports that reach into the forbidden directory
    # Match patterns like: from '../background/', from '../../background/', import('../background/')
    local matches
    matches=$(grep -rn "from ['\"].*/${forbidden_dir}/\|import(['\"].*/${forbidden_dir}/" "src/${from_dir}/" 2>/dev/null || true)

    if [ -n "$matches" ]; then
        echo "VIOLATION: src/${from_dir}/ imports from ${forbidden_dir}/"
        echo "$matches"
        echo ""
        VIOLATIONS=1
    fi
}

# content/ must not import background/
check_boundary "content" "background"

# inject/ must not import background/
check_boundary "inject" "background"

# inject/ must not import content/
check_boundary "inject" "content"

# lib/ must not import background/ or content/
check_boundary "lib" "background"
check_boundary "lib" "content"

if [ "$VIOLATIONS" -eq 1 ]; then
    echo "Import boundary violations found. Cross-module imports must go through lib/ or explicit interfaces."
    exit 1
fi

echo "All import boundaries respected"
