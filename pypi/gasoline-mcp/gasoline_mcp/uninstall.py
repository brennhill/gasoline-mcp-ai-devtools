"""Uninstall logic for Gasoline MCP CLI."""

import json
import os
from . import config


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

    result = {
        "success": False,
        "removed": [],
        "notConfigured": [],
        "errors": [],
    }

    candidates = config.get_config_candidates()

    for candidate_path in candidates:
        tool_name = config.get_tool_name_from_path(candidate_path)

        try:
            # Check if file exists
            if not os.path.exists(candidate_path):
                result["notConfigured"].append(tool_name)
                continue

            # Read config
            read_result = config.read_config_file(candidate_path)
            if not read_result["valid"]:
                result["errors"].append(f"{tool_name}: Invalid JSON, cannot uninstall")
                continue

            # Check if gasoline is configured
            cfg = read_result["data"]
            if not cfg.get("mcpServers", {}).get("gasoline"):
                result["notConfigured"].append(tool_name)
                continue

            if dry_run:
                result["removed"].append({
                    "name": tool_name,
                    "path": candidate_path,
                })
                if verbose:
                    print(f"[DEBUG] Would remove gasoline from {candidate_path}")
                continue

            # Remove gasoline entry
            modified = json.loads(json.dumps(cfg))  # Deep copy
            del modified["mcpServers"]["gasoline"]

            # Write back (or delete if no other servers)
            has_other_servers = len(modified["mcpServers"]) > 0
            if has_other_servers:
                config.write_config_file(candidate_path, modified, False)
            else:
                # No other servers, delete the file
                os.remove(candidate_path)

            result["removed"].append({
                "name": tool_name,
                "path": candidate_path,
            })

            if verbose:
                print(f"[DEBUG] Removed gasoline from {candidate_path}")

        except Exception as err:
            result["errors"].append(f"{tool_name}: {str(err)}")
            if verbose:
                print(f"[DEBUG] Error uninstalling from {tool_name}: {str(err)}")

    result["success"] = len(result["removed"]) > 0
    return result
