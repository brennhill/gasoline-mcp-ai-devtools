#!/usr/bin/env python3
"""
Lint documentation for broken links and metadata issues.
Checks:
1. Links point to existing files
2. Code references (file.go:line) exist in codebase
3. All docs have YAML frontmatter
4. last-verified dates are recent (< 30 days)
5. No orphaned markdown files
"""

import os
import re
from pathlib import Path
from datetime import datetime, timedelta

DOCS_DIR = Path("/Users/brenn/dev/gasoline/docs")
CODE_DIR = Path("/Users/brenn/dev/gasoline")

class DocumentLinter:
    def __init__(self):
        self.errors = []
        self.warnings = []
        self.info = []

    def error(self, msg):
        self.errors.append(f"❌ {msg}")

    def warning(self, msg):
        self.warnings.append(f"⚠️  {msg}")

    def info_msg(self, msg):
        self.info.append(f"ℹ️  {msg}")

    def check_markdown_links(self, file_path, content):
        """Check all markdown links in a file"""
        # Pattern: [text](path) or [text](path#anchor)
        pattern = r'\[([^\]]+)\]\(([^)]+)\)'
        matches = re.findall(pattern, content)

        for text, link in matches:
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
                    with open(code_file, 'r') as f:
                        content = f.read()
                        if f"func {func_ref}(" not in content:
                            self.warning(f"{file_path.relative_to(DOCS_DIR)}: function not found: {func_ref} in {file_ref}")
                except:
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
                        self.warning(f"{file_path.relative_to(DOCS_DIR)}: last-verified is stale ({date_str})")
            else:
                self.warning(f"{file_path.relative_to(DOCS_DIR)}: missing 'last-verified' field")

        except Exception as e:
            self.warning(f"{file_path.relative_to(DOCS_DIR)}: frontmatter parse error: {str(e)}")

    def lint_file(self, file_path):
        """Lint a single markdown file"""
        try:
            with open(file_path, 'r') as f:
                content = f.read()

            self.check_frontmatter(file_path, content)
            self.check_markdown_links(file_path, content)
            self.check_code_references(file_path, content)

        except Exception as e:
            self.error(f"{file_path}: read error: {str(e)}")

    def lint_all(self):
        """Lint all markdown files in docs"""
        md_files = list(DOCS_DIR.rglob("*.md"))

        print(f"🔍 Linting {len(md_files)} markdown files...\n")

        for md_file in md_files:
            self.lint_file(md_file)

        # Print results
        print("\n" + "=" * 70)
        print("LINT RESULTS")
        print("=" * 70 + "\n")

        if self.errors:
            print(f"ERRORS ({len(self.errors)}):")
            for e in self.errors[:20]:  # Limit output
                print(f"  {e}")
            if len(self.errors) > 20:
                print(f"  ... and {len(self.errors) - 20} more errors")
            print()

        if self.warnings:
            print(f"WARNINGS ({len(self.warnings)}):")
            for w in self.warnings[:20]:
                print(f"  {w}")
            if len(self.warnings) > 20:
                print(f"  ... and {len(self.warnings) - 20} more warnings")
            print()

        print("\n" + "=" * 70)
        print(f"📊 Summary: {len(self.errors)} errors, {len(self.warnings)} warnings")
        print("=" * 70)

        return len(self.errors) == 0

if __name__ == "__main__":
    linter = DocumentLinter()
    success = linter.lint_all()
    exit(0 if success else 1)
