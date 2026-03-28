"""PyPI wrapper entry point.

Purpose: Delegate CLI/binary execution to platform-aware runtime routing.
Why: Keeps `kaboom-agentic-browser` command behavior consistent across installed platforms.
Docs: docs/features/feature/enhanced-cli-config/index.md
"""

from .platform import run


def main():
    """Run the Kaboom Agentic Browser server."""
    run()


if __name__ == "__main__":
    main()
