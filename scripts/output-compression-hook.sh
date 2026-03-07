#!/usr/bin/env bash
# output-compression-hook.sh — Claude Code PostToolUse hook for Bash tool output compression.
# Detects test runner and build output, compresses verbose results to save context tokens.
#
# Usage: Configure in .claude/settings.json as a PostToolUse hook on Bash.
# The hook reads JSON input from stdin (Claude Code hook protocol).
#
# Output: JSON with additionalContext containing compressed results.
# Stats posted to daemon at POST /api/token-savings (best-effort).

set -euo pipefail

# Read hook input from stdin, write to temp file for python3 to process.
TMPFILE=$(mktemp)
trap 'rm -f "$TMPFILE"' EXIT
cat > "$TMPFILE"

# All logic in python3 — reads from temp file, outputs JSON or exits silently.
python3 - "$TMPFILE" << 'PYEOF'
import sys, json, re, os

input_file = sys.argv[1]

try:
    with open(input_file) as f:
        input_data = json.load(f)
except (json.JSONDecodeError, ValueError, FileNotFoundError):
    sys.exit(0)

# --- Early exits ---

tool_name = input_data.get("tool_name", "")
if tool_name != "Bash":
    sys.exit(0)

# Extract output from tool_response — handle string or dict.
tool_response = input_data.get("tool_response", "")
if isinstance(tool_response, dict):
    output = tool_response.get("output",
             tool_response.get("stdout",
             tool_response.get("content", "")))
elif isinstance(tool_response, str):
    output = tool_response
else:
    output = str(tool_response) if tool_response else ""

if not output or not output.strip():
    sys.exit(0)

lines = output.split("\n")
total_lines = len(lines)

# Extract command for pattern matching.
tool_input = input_data.get("tool_input", {})
command = tool_input.get("command", "") if isinstance(tool_input, dict) else ""


# --- Compression functions ---

def compress_go_test(lines, command):
    passed = []
    failed = []
    fail_details = {}  # test name -> list of error lines
    summary_lines = []
    durations = []
    current_run = None  # track current test name from === RUN
    pending_errors = []  # error lines between === RUN and --- FAIL

    for line in lines:
        stripped = line.strip()
        if stripped.startswith("=== RUN"):
            current_run = stripped.split()[-1] if len(stripped.split()) > 2 else None
            pending_errors = []
        elif stripped.startswith("--- PASS:"):
            passed.append(stripped)
            current_run = None
            pending_errors = []
        elif stripped.startswith("--- FAIL:"):
            failed.append(stripped)
            # Capture the first error line from pending_errors.
            if pending_errors:
                test_name = stripped
                fail_details[test_name] = pending_errors[:1]
            current_run = None
            pending_errors = []
        elif stripped.startswith("FAIL") and not stripped.startswith("--- FAIL:"):
            summary_lines.append(stripped)
        elif stripped.startswith("ok "):
            summary_lines.append(stripped)
            m = re.search(r'(\d+\.\d+s)', stripped)
            if m:
                durations.append(m.group(1))
        elif current_run and stripped and not stripped.startswith("==="):
            # Error/log line between === RUN and --- PASS/FAIL.
            pending_errors.append(stripped)

    if not passed and not failed and not summary_lines:
        return None

    result = [f"go test summary: {len(passed)} passed, {len(failed)} failed"]
    if durations:
        result.append(f"duration: {durations[-1]}")
    if failed:
        result.append("")
        result.append("FAILED TESTS:")
        for f in failed:
            result.append(f"  {f}")
            for detail in fail_details.get(f, []):
                result.append(f"    {detail}")
    if summary_lines:
        result.append("")
        for s in summary_lines:
            result.append(s)
    return "\n".join(result)


def compress_jest_vitest(lines, command):
    summary = []
    fail_files = []
    for line in lines:
        stripped = line.strip()
        if any(k in stripped for k in ("Test Suites:", "Tests:", "Snapshots:", "Time:")):
            summary.append(stripped)
        elif stripped.startswith("FAIL ") and "/" in stripped:
            fail_files.append(stripped)

    if not summary:
        return None

    result = ["jest/vitest summary:"]
    if fail_files:
        result.append("FAILURES:")
        result.extend(f"  {f}" for f in fail_files)
    result.extend(summary)
    return "\n".join(result)


def compress_pytest(lines, command):
    summary = []
    failures = []
    for line in lines:
        stripped = line.strip()
        if re.search(r'(\d+ passed|\d+ failed|\d+ error)', stripped):
            summary.append(stripped)
        elif stripped.startswith("FAILED "):
            failures.append(stripped)
        elif stripped.startswith("ERROR "):
            failures.append(stripped)

    if not summary and not failures:
        return None

    result = ["pytest summary:"]
    if failures:
        result.extend(failures)
    result.extend(summary)
    return "\n".join(result)


def compress_cargo_test(lines, command):
    summary = []
    failures = []
    for line in lines:
        stripped = line.strip()
        if stripped.startswith("test result:"):
            summary.append(stripped)
        elif stripped.startswith("test ") and "FAILED" in stripped:
            failures.append(stripped)

    if not summary:
        return None

    result = ["cargo test summary:"]
    if failures:
        result.append("FAILURES:")
        result.extend(f"  {f}" for f in failures)
    result.extend(summary)
    return "\n".join(result)


def compress_go_build(lines, command):
    errors = []
    for line in lines:
        stripped = line.strip()
        if re.match(r'.*\.go:\d+:\d+:.*', stripped):
            errors.append(stripped)
        elif stripped.startswith("#"):
            errors.append(stripped)

    if not errors:
        return None

    result = [f"go build/vet: {len(errors)} error(s):"]
    result.extend(errors)
    return "\n".join(result)


def compress_make(lines, command):
    important = []
    for line in lines:
        stripped = line.strip()
        if "Error" in stripped or "make: ***" in stripped:
            important.append(stripped)
        elif "warning:" in stripped.lower():
            important.append(stripped)

    if not important:
        return None

    result = [f"make: {len(important)} issue(s):"]
    result.extend(important)
    return "\n".join(result)


def compress_tsc(lines, command):
    errors = []
    for line in lines:
        stripped = line.strip()
        if "error TS" in stripped:
            errors.append(stripped)

    if not errors:
        return None

    result = [f"tsc: {len(errors)} error(s):"]
    result.extend(errors)
    return "\n".join(result)


def compress_npm_build(lines, command):
    errors = []
    for line in lines:
        stripped = line.strip()
        if "ERROR" in stripped or "Module not found" in stripped:
            errors.append(stripped)

    if not errors:
        return None

    result = [f"build: {len(errors)} error(s):"]
    result.extend(errors)
    return "\n".join(result)


def compress_cargo_build(lines, command):
    errors = []
    for line in lines:
        stripped = line.strip()
        if "error[E" in stripped:
            errors.append(stripped)

    if not errors:
        return None

    result = [f"cargo build: {len(errors)} error(s):"]
    result.extend(errors)
    return "\n".join(result)


def detect_and_compress(lines, command):
    """Returns (category, compressed) or (None, None)."""
    cmd_lower = command.lower()

    # --- Test runners ---
    if "go test" in cmd_lower or any(
        l.strip().startswith(("--- PASS:", "--- FAIL:", "ok \t"))
        for l in lines
    ):
        result = compress_go_test(lines, command)
        if result:
            return "test_output", result

    if any(x in cmd_lower for x in ("jest", "vitest")) or any(
        "Test Suites:" in l or "Tests:" in l for l in lines
    ):
        result = compress_jest_vitest(lines, command)
        if result:
            return "test_output", result

    if "pytest" in cmd_lower or any(
        re.search(r'\d+ passed', l) for l in lines[-20:]
    ):
        result = compress_pytest(lines, command)
        if result:
            return "test_output", result

    if "cargo test" in cmd_lower or any(
        l.strip().startswith("test result:") for l in lines
    ):
        result = compress_cargo_test(lines, command)
        if result:
            return "test_output", result

    # --- Build tools ---
    if any(x in cmd_lower for x in ("go build", "go vet")):
        result = compress_go_build(lines, command)
        if result:
            return "build_output", result

    if re.match(r'^make\b', cmd_lower):
        result = compress_make(lines, command)
        if result:
            return "build_output", result

    if "tsc" in cmd_lower:
        result = compress_tsc(lines, command)
        if result:
            return "build_output", result

    if any(x in cmd_lower for x in ("npm run build", "webpack")):
        result = compress_npm_build(lines, command)
        if result:
            return "build_output", result

    if "cargo build" in cmd_lower:
        result = compress_cargo_build(lines, command)
        if result:
            return "build_output", result

    return None, None


# --- Main logic ---

if total_lines < 50:
    sys.exit(0)

category, compressed = detect_and_compress(lines, command)

if compressed is None and total_lines <= 100:
    sys.exit(0)

if compressed is None and total_lines > 100:
    category = "generic_truncation"
    head = "\n".join(lines[:30])
    tail = "\n".join(lines[-20:])
    compressed = f"{head}\n\n...truncated ({total_lines} total lines)\n\n{tail}"

if compressed is None:
    sys.exit(0)

# Stats.
compressed_lines = len(compressed.split("\n"))
tokens_before = len(output) // 4
tokens_after = len(compressed) // 4

# Post stats to daemon for in-memory tracking (best-effort).
import urllib.request, urllib.error
port = os.environ.get("GASOLINE_PORT", "7890")
try:
    payload = json.dumps({
        "category": category,
        "tokens_before": tokens_before,
        "tokens_after": tokens_after
    }).encode()
    req = urllib.request.Request(
        f"http://127.0.0.1:{port}/api/token-savings",
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST"
    )
    urllib.request.urlopen(req, timeout=1)
except Exception:
    pass  # Daemon may not be running — that's fine.

# Output.
context = (
    f"[Output compressed: {total_lines} lines -> {compressed_lines} lines, "
    f"~{tokens_before} -> ~{tokens_after} tokens]\n\n{compressed}"
)
print(json.dumps({"additionalContext": context}))
PYEOF
