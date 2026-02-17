"""Output formatters for Gasoline MCP CLI."""


def success(message, details=""):
    """Format success message."""
    output = f"✅ {message}"
    if details:
        output += f"\n   {details}"
    return output


def error(message, recovery=""):
    """Format error message."""
    output = f"❌ {message}"
    if recovery:
        output += f"\n   {recovery}"
    return output


def warning(message, details=""):
    """Format warning message."""
    output = f"⚠️  {message}"
    if details:
        output += f"\n   {details}"
    return output


def info(message, details=""):
    """Format info message."""
    output = f"ℹ️  {message}"
    if details:
        output += f"\n   {details}"
    return output


def json_diff(before, after):
    """Format JSON diff for dry-run."""
    import json  # pylint: disable=import-outside-toplevel

    before_str = json.dumps(before, indent=2)
    after_str = json.dumps(after, indent=2)

    return f"ℹ️  Dry run: No files will be written\n\nBefore:\n{before_str}\n\nAfter:\n{after_str}"


def install_result(result):
    """Format install result."""
    output = ""
    installed = result.get("installed", result.get("updated", []))
    total = result.get("total", 5)

    if installed:
        output += f"✅ {len(installed)}/{total} clients updated:\n"
        for entry in installed:
            if entry.get("method") == "cli":
                output += f"   ✅ {entry['name']} (via CLI)\n"
            else:
                output += f"   ✅ {entry['name']} (at {entry['path']})\n"

    if result.get("errors", []):
        output += "\n❌ Errors:\n"
        for err in result["errors"]:
            if isinstance(err, dict):
                output += f"   ❌ {err['name']}: {err['message']}\n"
            else:
                output += f"   ❌ {err}\n"

    if result.get("notFound", []):
        output += f"\nℹ️  Not configured in: {', '.join(result['notFound'])}\n"

    return output


def _format_tool_ok(tool):
    """Format a tool with 'ok' status."""
    if tool.get("type") == "cli":
        return f"✅ {tool['name']}\n   Configured via CLI - Ready\n\n"
    return f"✅ {tool['name']}\n   {tool['path']} - Configured and ready\n\n"


def _format_tool_info(tool):
    """Format a tool with 'info' status (not detected)."""
    output = f"⚪ {tool['name']}\n"
    for issue in tool.get("issues", []):
        output += f"   {issue}\n"
    return output + "\n"


def _format_tool_problem(tool):
    """Format a tool with 'error' or 'warning' status."""
    icon = "❌" if tool["status"] == "error" else "⚠️ "
    fix_label = "Fix" if tool["status"] == "error" else "Suggestion"
    output = f"{icon} {tool['name']}\n"
    if tool.get("path"):
        output += f"   {tool['path']}\n"
    for issue in tool.get("issues", []):
        output += f"   Issue: {issue}\n"
    for suggestion in tool.get("suggestions", []):
        output += f"   {fix_label}: {suggestion}\n"
    return output + "\n"


def _format_binary(binary):
    """Format binary check section."""
    if binary.get("ok"):
        output = f"✅ Binary Check\n   Gasoline binary found at {binary['path']}\n"
        if binary.get("version"):
            output += f"   Version: {binary['version']}\n"
        return output
    return f"❌ Binary Check\n   {binary['error']}\n"


def diagnostic_report(report):
    """Format diagnostic report."""
    output = "\n📋 Gasoline MCP Diagnostic Report\n\n"

    for tool in report.get("tools", []):
        if tool["status"] == "ok":
            output += _format_tool_ok(tool)
        elif tool["status"] == "info":
            output += _format_tool_info(tool)
        else:
            output += _format_tool_problem(tool)

    if report.get("binary"):
        output += _format_binary(report["binary"])

    # Legacy path warnings
    if report.get("legacyWarnings"):
        output += "\n⚠️  Legacy Configs Found:\n"
        for w in report["legacyWarnings"]:
            output += f"   {w['description']}: {w['path']}\n"
            output += "   This path is no longer used. You can safely remove the gasoline entry.\n"

    if report.get("summary"):
        output += f"\n{report['summary']}\n"

    return output


def uninstall_result(result):
    """Format uninstall result."""
    output = ""

    if result.get("removed", []):
        count = len(result["removed"])
        output += f"✅ Removed from {count} client{'s' if count != 1 else ''}:\n"
        for entry in result["removed"]:
            if entry.get("method") == "cli":
                output += f"   ✅ {entry['name']} (via CLI)\n"
            else:
                output += f"   ✅ {entry['name']} (removed from {entry['path']})\n"
    else:
        output += "ℹ️  Gasoline not configured in any clients\n"

    if result.get("notConfigured", []):
        output += f"\nℹ️  Not configured in: {', '.join(result['notConfigured'])}\n"

    if result.get("errors", []):
        output += "\n❌ Errors:\n"
        for err in result["errors"]:
            output += f"   {err}\n"

    return output
