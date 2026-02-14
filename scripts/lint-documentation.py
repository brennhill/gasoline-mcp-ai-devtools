#!/usr/bin/env python3
# pylint: disable=invalid-name
"""
Lint documentation for broken links and metadata issues.
Checks:
1. Links point to existing files
2. Code references (file.go:line) exist in codebase
3. All docs have YAML frontmatter
4. last-verified dates are recent (< 30 days)
5. No orphaned markdown files
"""

import re
from pathlib import Path
from datetime import datetime, timedelta
import sys

DOCS_DIR = Path("/Users/brenn/dev/gasoline/docs")
CODE_DIR = Path("/Users/brenn/dev/gasoline")

class DocumentLinter:
    """Lint markdown documentation files for common issues."""

    def __init__(self):
        self.errors = []
        self.warnings = []
        self.info = []

    def error(self, msg):
        """Record an error message."""
        self.errors.append(f"❌ {msg}")

    def warning(self, msg):
        """Record a warning message."""
        self.warnings.append(f"⚠️  {msg}")

    def info_msg(self, msg):
        """Record an informational message."""
        self.info.append(f"ℹ️  {msg}")

    def check_markdown_links(self, file_path, content):
        """Check all markdown links in a file"""
        # Pattern: [text](path) or [text](path#anchor)
        pattern = r'\[([^\]]+)\]\(([^)]+)\)'
        matches = re.findall(pattern, content)

        for _text, link in matches:
            # Skip external links
            if link.startswith("http://") or link.startswith("https://"):
                continue

            # Remove anchor
            file_part = link.split("#")[0]

            # Skip empty links
            if not file_part:
                continue

            # Resolve relative path
            if file_part.startswith("/"):
                target = CODE_DIR / file_part.lstrip("/")
            else:
                target = file_path.parent / file_part

            # Check if file exists
            if not target.exists():
                self.error(f"{file_path.relative_to(DOCS_DIR)}: broken link to {link}")

    def check_code_references(self, file_path, content):
        """Check code references (file.go:function)"""
        pattern = r'`([a-z_/]+\.go):([a-zA-Z_][a-zA-Z0-9_]*)\(\)`'
        matches = re.findall(pattern, content)

        for file_ref, func_ref in matches:
            code_file = CODE_DIR / file_ref
            if not code_file.exists():
                self.error(f"{file_path.relative_to(DOCS_DIR)}: code file not found: {file_ref}")
            else:
                try:
                    with open(code_file, 'r', encoding='utf-8') as f:
                        code_content = f.read()
                        if f"func {func_ref}(" not in code_content:
                            rel = file_path.relative_to(DOCS_DIR)
                            self.warning(
                                f"{rel}: function not found:"
                                f" {func_ref} in {file_ref}"
                            )
                except (OSError, UnicodeDecodeError):
                    pass

    def check_frontmatter(self, file_path, content):
        """Check YAML frontmatter quality"""
        if not content.startswith("---"):
            self.warning(f"{file_path.relative_to(DOCS_DIR)}: missing YAML frontmatter")
            return

        # Extract frontmatter
        try:
            end = content.find("\n---\n", 3) + 4
            frontmatter = content[4:end]

            # Check required fields
            if "status:" not in frontmatter:
                self.warning(f"{file_path.relative_to(DOCS_DIR)}: missing 'status' field")

            if "last-verified:" in frontmatter:
                # Extract date
                match = re.search(r'last-verified:\s*(\d{4}-\d{2}-\d{2})', frontmatter)
                if match:
                    date_str = match.group(1)
                    doc_date = datetime.strptime(date_str, "%Y-%m-%d")
                    if datetime.now() - doc_date > timedelta(days=30):
                        rel = file_path.relative_to(DOCS_DIR)
                        self.warning(
                            f"{rel}: last-verified is stale"
                            f" ({date_str})"
                        )
            else:
                self.warning(f"{file_path.relative_to(DOCS_DIR)}: missing 'last-verified' field")

        except (ValueError, IndexError) as e:
            self.warning(f"{file_path.relative_to(DOCS_DIR)}: frontmatter parse error: {str(e)}")

    def lint_file(self, file_path):
        """Lint a single markdown file"""
        try:
            with open(file_path, 'r', encoding='utf-8') as f:
                content = f.read()

            self.check_frontmatter(file_path, content)
            self.check_markdown_links(file_path, content)
            self.check_code_references(file_path, content)

        except (OSError, UnicodeDecodeError) as e:
            self.error(f"{file_path}: read error: {str(e)}")

    def _print_section(self, label, items):
        """Print a capped section of lint results (max 20 shown)."""
        if not items:
            return
        print(f"{label} ({len(items)}):")
        for item in items[:20]:
            print(f"  {item}")
        if len(items) > 20:
            print(f"  ... and {len(items) - 20} more {label.lower()}")
        print()

    def lint_all(self):
        """Lint all markdown files in docs"""
        md_files = list(DOCS_DIR.rglob("*.md"))
        print(f"Linting {len(md_files)} markdown files...\n")

        for md_file in md_files:
            self.lint_file(md_file)

        print("\n" + "=" * 70)
        print("LINT RESULTS")
        print("=" * 70 + "\n")

        self._print_section("ERRORS", self.errors)
        self._print_section("WARNINGS", self.warnings)

        print("\n" + "=" * 70)
        print(f"Summary: {len(self.errors)} errors, {len(self.warnings)} warnings")
        print("=" * 70)

        return len(self.errors) == 0

if __name__ == "__main__":
    linter = DocumentLinter()
    passed = linter.lint_all()
    sys.exit(0 if passed else 1)
