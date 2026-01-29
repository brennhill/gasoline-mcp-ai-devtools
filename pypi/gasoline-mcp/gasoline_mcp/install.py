"""Install logic for Gasoline MCP CLI."""

import json
from . import config, output


def generate_default_config():
    """Generate default MCP config for gasoline."""
    return {
        "mcpServers": {
            "gasoline": {
                "command": "gasoline-mcp",
                "args": [],
            },
        },
    }


def execute_install(options=None):
    """Execute install operation.

    Args:
        options: dict with {dryRun, forAll, envVars, verbose}

    Returns:
        dict: {success, updated, errors, total}
    """
    options = options or {}
    dry_run = options.get("dryRun", False)
    for_all = options.get("forAll", False)
    env_vars = options.get("envVars", {})
    verbose = options.get("verbose", False)

    result = {
        "success": False,
        "updated": [],
        "errors": [],
        "diffs": [],
        "total": 4,
    }

    candidates = config.get_config_candidates()
    gasoline_entry = {
        "command": "gasoline-mcp",
        "args": [],
    }

    found_existing = False

    for candidate_path in candidates:
        tool_name = config.get_tool_name_from_path(candidate_path)

        try:
            # Try to read existing config
            read_result = config.read_config_file(candidate_path)
            is_new = False

            if read_result["valid"]:
                config_data = read_result["data"]
                found_existing = True
            else:
                # File doesn't exist, create new config
                config_data = generate_default_config()
                is_new = True

            # Merge gasoline config
            before = json.loads(json.dumps(config_data))  # Deep copy
            merged = config.merge_gasoline_config(
                config_data, gasoline_entry, env_vars
            )

            # Write config
            write_result = config.write_config_file(candidate_path, merged, dry_run)

            result["updated"].append({
                "name": tool_name,
                "path": candidate_path,
                "isNew": is_new,
            })

            if dry_run and write_result.get("before"):
                result["diffs"].append({
                    "path": candidate_path,
                    "before": before,
                    "after": merged,
                })

            if verbose:
                action = "Created" if is_new else "Updated"
                print(f"[DEBUG] {action}: {candidate_path}")

            # Stop at first match if not --for-all
            if not for_all and found_existing:
                break

        except Exception as err:
            # If file doesn't exist and we're not --for-all, continue
            import os
            if not os.path.exists(candidate_path) and not for_all:
                continue

            result["errors"].append({
                "name": tool_name,
                "message": str(err),
                "recovery": getattr(err, "recovery", ""),
            })

            if verbose:
                print(f"[DEBUG] Error on {candidate_path}: {str(err)}")

            # If --for-all, continue even on error
            if not for_all:
                break

    # If no existing config found and --for-all not set, try default
    if not found_existing and not for_all and len(result["updated"]) == 0:
        try:
            default_path = candidates[0]
            merged = config.merge_gasoline_config(
                generate_default_config(), gasoline_entry, env_vars
            )
            config.write_config_file(default_path, merged, dry_run)
            result["updated"].append({
                "name": config.get_tool_name_from_path(default_path),
                "path": default_path,
                "isNew": True,
            })
        except Exception as err:
            result["errors"].append({
                "name": "Claude Desktop",
                "message": str(err),
                "recovery": getattr(err, "recovery", ""),
            })

    result["success"] = len(result["updated"]) > 0
    return result
