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

    if result.get("updated", []):
        output += f"✅ {len(result['updated'])}/{result['total']} tools updated:\n"
        for tool in result["updated"]:
            output += f"   ✅ {tool['name']} (at {tool['path']})\n"

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
    return f"✅ {tool['name']}\n   {tool['path']} - Configured and ready\n\n"


def _format_tool_problem(tool):
    """Format a tool with 'error' or 'warning' status."""
    icon = "❌" if tool["status"] == "error" else "⚠️ "
    fix_label = "Fix" if tool["status"] == "error" else "Suggestion"
    output = f"{icon} {tool['name']}\n   {tool['path']}\n"
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
        else:
            output += _format_tool_problem(tool)

    if report.get("binary"):
        output += _format_binary(report["binary"])

    if report.get("summary"):
        output += f"\n{report['summary']}\n"

    return output


def uninstall_result(result):
    """Format uninstall result."""
    output = ""

    if result.get("removed", []):
        count = len(result["removed"])
        output += f"✅ Removed from {count} tool{'s' if count != 1 else ''}:\n"
        for tool in result["removed"]:
            output += f"   ✅ {tool['name']} (removed from {tool['path']})\n"
    else:
        output += "ℹ️  Gasoline not configured in any tools\n"

    if result.get("notConfigured", []):
        output += f"\nℹ️  Not configured in: {', '.join(result['notConfigured'])}\n"

    if result.get("errors", []):
        output += "\n❌ Errors:\n"
        for err in result["errors"]:
            output += f"   {err}\n"

    return output
