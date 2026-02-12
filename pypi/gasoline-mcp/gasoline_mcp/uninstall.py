# pylint: disable=duplicate-code
"""Uninstall logic for Gasoline MCP CLI."""

import json
import os
from . import config


def _uninstall_candidate(candidate_path, dry_run, verbose):
    """Uninstall gasoline from a single candidate config path.

    Returns:
        tuple: (category, entry) where category is 'removed', 'notConfigured', or 'error'.
    """
    tool_name = config.get_tool_name_from_path(candidate_path)

    if not os.path.exists(candidate_path):
        return "notConfigured", tool_name

    read_result = config.read_config_file(candidate_path)
    if not read_result["valid"]:
        return "error", f"{tool_name}: Invalid JSON, cannot uninstall"

    cfg = read_result["data"]
    if not cfg.get("mcpServers", {}).get("gasoline"):
        return "notConfigured", tool_name

    entry = {"name": tool_name, "path": candidate_path}

    if dry_run:
        if verbose:
            print(f"[DEBUG] Would remove gasoline from {candidate_path}")
        return "removed", entry

    modified = json.loads(json.dumps(cfg))
    del modified["mcpServers"]["gasoline"]

    if modified["mcpServers"]:
        config.write_config_file(candidate_path, modified, False)
    else:
        os.remove(candidate_path)

    if verbose:
        print(f"[DEBUG] Removed gasoline from {candidate_path}")
    return "removed", entry


def execute_uninstall(options=None):
    """Execute uninstall operation.

    Args:
        options: dict with {dryRun, verbose}

    Returns:
        dict: {success, removed, notConfigured, errors}
    """
    options = options or {}
    dry_run = options.get("dryRun", False)
    verbose = options.get("verbose", False)

    result = {"success": False, "removed": [], "notConfigured": [], "errors": []}

    for candidate_path in config.get_config_candidates():
        try:
            category, entry = _uninstall_candidate(candidate_path, dry_run, verbose)
            result[category].append(entry)
        except (OSError, KeyError, ValueError) as err:
            tool_name = config.get_tool_name_from_path(candidate_path)
            result["errors"].append(f"{tool_name}: {str(err)}")
            if verbose:
                print(f"[DEBUG] Error uninstalling from {tool_name}: {str(err)}")

    result["success"] = len(result["removed"]) > 0
    return result
