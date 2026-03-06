#!/usr/bin/env bash
# quality-gate-hook.sh — Claude Code PostToolUse hook for quality gate enforcement.
# Reads the changed file, runs static checks, injects findings + standards into context.
#
# Usage: Configure in .claude/settings.json as a PostToolUse hook on Edit|Write.
# The hook reads JSON input from stdin (Claude Code hook protocol).
#
# Output: JSON with additionalContext containing:
#   1. Standards doc (from .gasoline.json -> code_standards)
#   2. File size warning (if approaching limit)
#   3. jscpd duplicate detection results (if npx available)
#   4. Review instructions for the primary model

set -euo pipefail

# Read hook input from stdin.
INPUT=$(cat)

# Extract the changed file path from the hook input.
FILE_PATH=$(echo "$INPUT" | python3 -c "
import sys, json
data = json.load(sys.stdin)
# Edit tool uses tool_input.file_path, Write uses tool_input.file_path
ti = data.get('tool_input', {})
print(ti.get('file_path', ''))
" 2>/dev/null || echo "")

if [ -z "$FILE_PATH" ] || [ ! -f "$FILE_PATH" ]; then
    # No file path or file doesn't exist — pass through silently.
    exit 0
fi

# Find project root by walking up from the file looking for .gasoline.json.
find_project_root() {
    local dir
    dir=$(dirname "$FILE_PATH")
    while [ "$dir" != "/" ]; do
        if [ -f "$dir/.gasoline.json" ]; then
            echo "$dir"
            return 0
        fi
        dir=$(dirname "$dir")
    done
    return 1
}

PROJECT_ROOT=$(find_project_root) || exit 0  # No .gasoline.json = no quality gates.

# Read config.
CONFIG_FILE="$PROJECT_ROOT/.gasoline.json"
CODE_STANDARDS=$(python3 -c "
import json
with open('$CONFIG_FILE') as f:
    cfg = json.load(f)
print(cfg.get('code_standards', 'gasoline-code-standards.md'))
" 2>/dev/null || echo "gasoline-code-standards.md")

FILE_SIZE_LIMIT=$(python3 -c "
import json
with open('$CONFIG_FILE') as f:
    cfg = json.load(f)
print(cfg.get('file_size_limit', 800))
" 2>/dev/null || echo "800")

STANDARDS_PATH="$PROJECT_ROOT/$CODE_STANDARDS"

# Build context parts.
CONTEXT=""

# 1. Standards doc (truncated to avoid blowing up context).
if [ -f "$STANDARDS_PATH" ]; then
    STANDARDS_CONTENT=$(head -150 "$STANDARDS_PATH")
    CONTEXT="$CONTEXT
=== PROJECT CODE STANDARDS ===
$STANDARDS_CONTENT
=== END STANDARDS ===
"
fi

# 2. File size check.
if [ -f "$FILE_PATH" ]; then
    LINE_COUNT=$(wc -l < "$FILE_PATH" | tr -d ' ')
    if [ "$LINE_COUNT" -gt "$FILE_SIZE_LIMIT" ]; then
        CONTEXT="$CONTEXT
WARNING: $FILE_PATH is $LINE_COUNT lines (limit: $FILE_SIZE_LIMIT). This file must be split.
"
    elif [ "$LINE_COUNT" -gt $((FILE_SIZE_LIMIT * 90 / 100)) ]; then
        CONTEXT="$CONTEXT
NOTE: $FILE_PATH is $LINE_COUNT lines (limit: $FILE_SIZE_LIMIT). Approaching the limit — consider splitting.
"
    fi
fi

# 3. Duplicate detection (jscpd) — only for source files, only if npx is available.
FILE_EXT="${FILE_PATH##*.}"
FILE_DIR=$(dirname "$FILE_PATH")
case "$FILE_EXT" in
    go|ts|js|tsx|jsx|py|rs)
        if command -v npx &>/dev/null; then
            DUPES=$(npx --yes jscpd "$FILE_DIR" --min-lines 8 --min-tokens 60 --reporters consoleFull --silent 2>/dev/null | tail -20 || true)
            if [ -n "$DUPES" ] && echo "$DUPES" | grep -q "Clone found"; then
                CONTEXT="$CONTEXT
=== DUPLICATE CODE DETECTED ===
$DUPES
=== END DUPLICATES ===
"
            fi
        fi
        ;;
esac

# 4. Review instruction.
if [ -n "$CONTEXT" ]; then
    CONTEXT="$CONTEXT
QUALITY GATE: Review your change against the standards above. Fix any violations before proceeding. If duplicates were found, extract to a shared helper or document why duplication is intentional.
"
fi

# Output as JSON with additionalContext (Claude Code hook protocol).
if [ -n "$CONTEXT" ]; then
    # Escape for JSON.
    ESCAPED=$(python3 -c "
import sys, json
print(json.dumps(sys.stdin.read()))
" <<< "$CONTEXT")
    echo "{\"additionalContext\": $ESCAPED}"
fi
