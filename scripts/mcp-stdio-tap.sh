#!/bin/sh
set -eu

IN_PATH="${GASOLINE_MCP_STDIO_TAP_IN:-/tmp/gasoline-mcp-stdio.in}"
OUT_PATH="${GASOLINE_MCP_STDIO_TAP_OUT:-/tmp/gasoline-mcp-stdio.out}"
BIN_PATH="/Users/brenn/dev/gasoline/gasoline-mcp"

exec 3> "$IN_PATH"
exec 4> "$OUT_PATH"

tee /dev/fd/3 | "$BIN_PATH" | tee /dev/fd/4
