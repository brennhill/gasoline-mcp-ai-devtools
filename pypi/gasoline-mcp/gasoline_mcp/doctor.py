"""Doctor diagnostics for Gasoline MCP CLI."""

import os
import subprocess
from . import config, output


def test_binary():
    """Test if gasoline binary is available and working.

    Returns:
        dict: {ok: bool, path?: str, version?: str, error?: str}
    """
    try:
        from .platform import get_binary_path

        try:
            binary_path = get_binary_path()

            # Test binary with --version
            try:
                result = subprocess.run(
                    [binary_path, "--version"],
                    capture_output=True,
                    text=True,
                    timeout=5,
                )
                version = result.stdout.strip() or "unknown"

                return {
                    "ok": True,
                    "path": binary_path,
                    "version": version,
                }
            except Exception as e:
                return {
                    "ok": False,
                    "path": binary_path,
                    "error": f"Binary found but failed to execute: {e}",
                }

        except Exception as e:
            return {
                "ok": False,
                "error": str(e),
            }

    except Exception as err:
        return {
            "ok": False,
            "error": f"Error testing binary: {err}",
        }


def run_diagnostics(verbose=False):
    """Run full diagnostics on all config locations.

    Args:
        verbose: If True, log debug info

    Returns:
        dict: Diagnostic report with tools array and summary
    """
    candidates = config.get_config_candidates()
    tools = []

    # Check each config location
    for candidate_path in candidates:
        tool_name = config.get_tool_name_from_path(candidate_path)
        tool = {
            "name": tool_name,
            "path": candidate_path,
            "status": "error",
            "issues": [],
            "suggestions": [],
        }

        if verbose:
            print(f"[DEBUG] Checking {tool_name} at {candidate_path}")

        # Check if file exists
        if not os.path.exists(candidate_path):
            tool["status"] = "info"
            tool["issues"].append("Config file not found")
            tool["suggestions"].append("Run: gasoline-mcp --install --for-all")
            tools.append(tool)
            continue

        # Try to read and validate
        read_result = config.read_config_file(candidate_path)
        if not read_result["valid"]:
            tool["status"] = "error"
            tool["issues"].append("Invalid JSON")
            tool["suggestions"].append("Fix the JSON syntax or run: gasoline-mcp --install")
            tools.append(tool)
            continue

        # Check if gasoline entry exists
        cfg = read_result["data"]
        if not cfg.get("mcpServers", {}).get("gasoline"):
            tool["status"] = "error"
            tool["issues"].append("gasoline entry missing from mcpServers")
            tool["suggestions"].append("Run: gasoline-mcp --install --for-all")
            tools.append(tool)
            continue

        # All checks passed
        tool["status"] = "ok"
        tools.append(tool)

    # Check binary availability
    binary = test_binary()

    # Generate summary
    ok_count = len([t for t in tools if t["status"] == "ok"])
    error_count = len([t for t in tools if t["status"] == "error"])
    info_count = len([t for t in tools if t["status"] == "info"])

    summary = f"Summary: {ok_count} tool{'s' if ok_count != 1 else ''} ready"
    if error_count > 0:
        summary += f", {error_count} need{'s' if error_count == 1 else ''} repair"
    if info_count > 0:
        summary += f", {info_count} not configured"

    return {
        "tools": tools,
        "binary": binary,
        "summary": summary,
    }
