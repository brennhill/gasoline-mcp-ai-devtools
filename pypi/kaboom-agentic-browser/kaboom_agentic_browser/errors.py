# pylint: disable=duplicate-code
"""Custom error classes for the PyPI wrapper.

Purpose: Standardize user-facing error and recovery output across commands.
Why: Keeps diagnostics and install failures understandable and fixable.
Docs: docs/features/feature/enhanced-cli-config/index.md
"""

import os


class KaboomError(Exception):
    """Base class for all Kaboom errors."""

    def __init__(self, message, recovery=""):
        self.message = message
        self.recovery = recovery
        self.name = "KaboomError"
        super().__init__(message)

    def format(self):
        """Format error with recovery suggestion."""
        output = f"❌ {self.message}"
        if self.recovery:
            output += f"\n   {self.recovery}"
        return output


class FilePermissionError(KaboomError):
    """Raised when permission is denied."""

    def __init__(self, path):
        msg = f"Permission denied writing {path}"
        recovery = (
            "Try: sudo kaboom-agentic-browser --install\n"
            f"Or: Check permissions with: ls -la {os.path.dirname(path)}"
        )
        super().__init__(msg, recovery)
        self.name = "FilePermissionError"


class InvalidJSONError(KaboomError):
    """Raised when JSON parsing fails."""

    def __init__(self, path, line_number=None, error_message=""):
        msg = f"Invalid JSON in {path}"
        if line_number:
            msg += f" at line {line_number}"
        if error_message:
            msg += f"\n   {error_message}"
        recovery = (
            f"Fix options:\n   1. Manually edit: code {path}"
            "\n   2. Restore from backup and try --install again"
            "\n   3. Run: kaboom-agentic-browser --doctor (for more info)"
        )
        super().__init__(msg, recovery)
        self.name = "InvalidJSONError"


class BinaryNotFoundError(KaboomError):
    """Raised when binary is not found."""

    def __init__(self, expected_path):
        msg = f"Kaboom binary not found at {expected_path}"
        recovery = (
            "Reinstall: pip install -U kaboom-agentic-browser\n"
            "Or build from source: go build ./cmd/browser-agent"
        )
        super().__init__(msg, recovery)
        self.name = "BinaryNotFoundError"


class InvalidEnvFormatError(KaboomError):
    """Raised when environment variable format is invalid."""

    def __init__(self, env_str):
        msg = f'Invalid env format "{env_str}". Expected: KEY=VALUE'
        recovery = (
            "Examples of valid formats:\n   - --env DEBUG=1"
            "\n   - --env KABOOM_SERVER=http://localhost:7890"
            "\n   - --env LOG_LEVEL=info"
        )
        super().__init__(msg, recovery)
        self.name = "InvalidEnvFormatError"


class EnvWithoutInstallError(KaboomError):
    """Raised when --env is used without --install."""

    def __init__(self):
        msg = "--env only works with --install"
        recovery = "Usage: kaboom-agentic-browser --install --env KEY=VALUE"
        super().__init__(msg, recovery)
        self.name = "EnvWithoutInstallError"


class ConfigValidationError(KaboomError):
    """Raised when config validation fails."""

    def __init__(self, errors):
        msg = f"Config validation failed: {', '.join(errors)}"
        recovery = "Ensure config has mcpServers object with valid structure"
        super().__init__(msg, recovery)
        self.name = "ConfigValidationError"


class FileSizeError(KaboomError):
    """Raised when file exceeds size limit."""

    def __init__(self, path, size):
        msg = f"File {path} is too large ({size} bytes, max 1MB)"
        recovery = (
            "The config file is too large."
            " Please reduce its size or delete it and reinstall."
        )
        super().__init__(msg, recovery)
        self.name = "FileSizeError"
