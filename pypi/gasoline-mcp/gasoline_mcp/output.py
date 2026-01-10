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
    import json

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


def diagnostic_report(report):
    """Format diagnostic report."""
    output = "\n📋 Gasoline MCP Diagnostic Report\n\n"

    for tool in report.get("tools", []):
        if tool["status"] == "ok":
            output += f"✅ {tool['name']}\n"
            output += f"   {tool['path']} - Configured and ready\n\n"
        elif tool["status"] == "error":
            output += f"❌ {tool['name']}\n"
            output += f"   {tool['path']}\n"
            if tool.get("issues"):
                for issue in tool["issues"]:
                    output += f"   Issue: {issue}\n"
            if tool.get("suggestions"):
                for suggestion in tool["suggestions"]:
                    output += f"   Fix: {suggestion}\n"
            output += "\n"
        elif tool["status"] == "warning":
            output += f"⚠️  {tool['name']}\n"
            output += f"   {tool['path']}\n"
            if tool.get("issues"):
                for issue in tool["issues"]:
                    output += f"   Issue: {issue}\n"
            if tool.get("suggestions"):
                for suggestion in tool["suggestions"]:
                    output += f"   Suggestion: {suggestion}\n"
            output += "\n"

    if report.get("binary"):
        binary = report["binary"]
        if binary.get("ok"):
            output += "✅ Binary Check\n"
            output += f"   Gasoline binary found at {binary['path']}\n"
            if binary.get("version"):
                output += f"   Version: {binary['version']}\n"
        else:
            output += "❌ Binary Check\n"
            output += f"   {binary['error']}\n"

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
