"""Install logic for Gasoline MCP CLI."""

import json
from . import config


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


def _install_candidate(candidate_path, gasoline_entry, env_vars, dry_run, verbose):
    """Install gasoline config to a single candidate path.

    Returns:
        tuple: (update_entry, diff_entry_or_None, is_existing)
        Raises OSError/ValueError/KeyError on failure.
    """
    tool_name = config.get_tool_name_from_path(candidate_path)
    read_result = config.read_config_file(candidate_path)

    if read_result["valid"]:
        config_data = read_result["data"]
        is_new = False
    else:
        config_data = generate_default_config()
        is_new = True

    before = json.loads(json.dumps(config_data))
    merged = config.merge_gasoline_config(config_data, gasoline_entry, env_vars)
    write_result = config.write_config_file(candidate_path, merged, dry_run)

    update_entry = {"name": tool_name, "path": candidate_path, "isNew": is_new}

    diff_entry = None
    if dry_run and write_result.get("before"):
        diff_entry = {"path": candidate_path, "before": before, "after": merged}

    if verbose:
        action = "Created" if is_new else "Updated"
        print(f"[DEBUG] {action}: {candidate_path}")

    return update_entry, diff_entry, not is_new


def _handle_install_error(candidate_path, err, for_all, verbose):
    """Handle an install error for a candidate path.

    Returns:
        error_entry or None (None means skip silently).
    """
    import os  # pylint: disable=import-outside-toplevel
    if not os.path.exists(candidate_path) and not for_all:
        return None

    if verbose:
        print(f"[DEBUG] Error on {candidate_path}: {str(err)}")

    return {
        "name": config.get_tool_name_from_path(candidate_path),
        "message": str(err),
        "recovery": getattr(err, "recovery", ""),
    }


def _install_default_candidate(candidates, gasoline_entry, env_vars, dry_run):
    """Fallback: install to the first candidate when none existed.

    Returns:
        tuple: (update_entry_or_None, error_entry_or_None)
    """
    try:
        default_path = candidates[0]
        merged = config.merge_gasoline_config(
            generate_default_config(), gasoline_entry, env_vars
        )
        config.write_config_file(default_path, merged, dry_run)
        return {
            "name": config.get_tool_name_from_path(default_path),
            "path": default_path,
            "isNew": True,
        }, None
    except (OSError, ValueError, KeyError) as err:
        return None, {
            "name": "Claude Desktop",
            "message": str(err),
            "recovery": getattr(err, "recovery", ""),
        }


def _process_candidate(  # pylint: disable=too-many-arguments,too-many-positional-arguments
    candidate_path, gasoline_entry, env_vars, dry_run, verbose, for_all, result
):
    """Process a single candidate path during install.

    Returns:
        tuple: (found_existing: bool, should_stop: bool)
    """
    try:
        update, diff, is_existing = _install_candidate(
            candidate_path, gasoline_entry, env_vars, dry_run, verbose
        )
        result["updated"].append(update)
        if diff:
            result["diffs"].append(diff)
        should_stop = not for_all and is_existing
        return is_existing, should_stop
    except (OSError, ValueError, KeyError) as err:
        error_entry = _handle_install_error(candidate_path, err, for_all, verbose)
        if error_entry:
            result["errors"].append(error_entry)
            return False, not for_all
        return False, False


def _apply_fallback(candidates, gasoline_entry, env_vars, dry_run, result):
    """Apply fallback install to first candidate when none existed."""
    update, error = _install_default_candidate(
        candidates, gasoline_entry, env_vars, dry_run
    )
    if update:
        result["updated"].append(update)
    if error:
        result["errors"].append(error)


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

    result = {"success": False, "updated": [], "errors": [], "diffs": [], "total": 4}
    gasoline_entry = {"command": "gasoline-mcp", "args": []}
    candidates = config.get_config_candidates()
    found_existing = False

    for candidate_path in candidates:
        is_existing, should_stop = _process_candidate(
            candidate_path, gasoline_entry, env_vars, dry_run, verbose, for_all, result
        )
        found_existing = found_existing or is_existing
        if should_stop:
            break

    if not found_existing and not for_all and not result["updated"]:
        _apply_fallback(candidates, gasoline_entry, env_vars, dry_run, result)

    result["success"] = len(result["updated"]) > 0
    return result
