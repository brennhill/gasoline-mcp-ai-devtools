#!/usr/bin/env python3
"""
Standardize ALL markdown filenames to lowercase-with-hyphens.
This script:
1. Finds all .md files with uppercase/underscore patterns
2. Renames them to lowercase-with-hyphens
3. Updates all references in the codebase
4. Reports what changed
"""

import os
import re
from pathlib import Path
from collections import defaultdict

# Define the conversion rules
STANDARD_RENAMES = {
    # Spec files
    'PRODUCT_SPEC.md': 'product-spec.md',
    'TECH_SPEC.md': 'tech-spec.md',
    'QA_PLAN.md': 'qa-plan.md',

    # Common patterns
    'FEATURE_PROPOSAL.md': 'feature-proposal.md',
    'FEATURE_TRACKING.md': 'feature-tracking.md',
    'FEATURE_INDEX.md': 'feature-index.md',
    'FEATURE-INDEX.md': 'feature-index.md',
    'FEATURE-NAVIGATION.md': 'feature-navigation.md',
    'FEATURE_NAVIGATION.md': 'feature-navigation.md',

    # Review files
    'REVIEW.md': 'review.md',
    'BEHAVIORAL_BASELINES_REVIEW.md': 'behavioral-baselines-review.md',
    'GASOLINE_CI_REVIEW.md': 'gasoline-ci-review.md',
    'INTERCEPTION_DEFERRAL_REVIEW.md': 'interception-deferral-review.md',
    'MCP_TOOL_DESCRIPTIONS_REVIEW.md': 'mcp-tool-descriptions-review.md',
    'NOISE_FILTERING_REVIEW.md': 'noise-filtering-review.md',
    'PERFORMANCE_BUDGET_REVIEW.md': 'performance-budget-review.md',
    'RATE_LIMITING_REVIEW.md': 'rate-limiting-review.md',
    'SARIF_EXPORT_REVIEW.md': 'sarif-export-review.md',
    'SARIF_EXPORT_REVIEW_CRITICAL_ISSUES.md': 'sarif-export-review-critical-issues.md',
    'SARIF_EXPORT_REVIEW_RECOMMENDATIONS.md': 'sarif-export-review-recommendations.md',
    'SELF_TESTING_REVIEW.md': 'self-testing-review.md',
    'SPA_ROUTE_MEASUREMENT_REVIEW.md': 'spa-route-measurement-review.md',

    # Other patterns
    'MIGRATION.md': 'migration.md',
    'UAT_GUIDE.md': 'uat-guide.md',
    'RECORDING_SCENARIOS.md': 'recording-scenarios.md',
    'IMPLEMENTATION_PLAN.md': 'implementation-plan.md',
    'REVIEW_SUMMARY.md': 'review-summary.md',
    'BUSINESS_PITCH.md': 'business-pitch.md',
    'EXECUTIVE_SUMMARY.md': 'executive-summary.md',
    'COMMIT_MESSAGE.md': 'commit-message.md',
    'COMPETITIVE_ADVANTAGE.md': 'competitive-advantage.md',
    'QUESTIONS.md': 'questions.md',
    'STATUS.md': 'status.md',
    'QUICK_START.md': 'quick-start.md',
    'VALIDATION_GUIDE.md': 'validation-guide.md',
    'VALIDATION_PLAN.md': 'validation-plan.md',
    'WAKE_UP.md': 'wake-up.md',

    # Root level docs
    'DEVELOPMENT.md': 'development.md',
    'FILE-ORGANIZATION-SUMMARY.md': 'file-organization-summary.md',
    'STARTUP-OPTIMIZATION-COMPLETE.md': 'startup-optimization-complete.md',
    'V6-TESTSPRITE-COMPETITION.md': 'v6-testsprite-competition.md',
    'NETWORK_CAPTURE.md': 'network-capture.md',

    # .claude/ docs
    'INSTRUCTIONS.md': 'instructions.md',
    'TDD-ENFORCEMENT-SUMMARY.md': 'tdd-enforcement-summary.md',

    # Archive files (sample - will handle pattern-based)
}

def convert_name(filename):
    """Convert filename to lowercase-with-hyphens."""
    if filename in STANDARD_RENAMES:
        return STANDARD_RENAMES[filename]

    # Pattern-based conversion for archive files
    if '_' in filename:
        converted = filename.replace('_', '-').lower()
        return converted

    return filename.lower()

def find_all_markdown_files(root_dir):
    """Find all markdown files that need renaming."""
    files_to_rename = []

    for dirpath, dirnames, filenames in os.walk(root_dir):
        # Skip node_modules, .git, build artifacts
        if any(skip in dirpath for skip in ['node_modules', '.git', 'pypi', '.next']):
            continue

        for filename in filenames:
            if not filename.endswith('.md'):
                continue

            # Skip README.md and CHANGELOG.md (standard)
            if filename in ['README.md', 'CHANGELOG.md']:
                continue

            # Skip templates
            if 'templates' in dirpath and filename.endswith('-TEMPLATE.md'):
                continue

            # Check if needs renaming
            new_name = convert_name(filename)
            if new_name != filename:
                full_path = os.path.join(dirpath, filename)
                files_to_rename.append({
                    'old_path': full_path,
                    'old_name': filename,
                    'new_name': new_name,
                    'dir': dirpath
                })

    return files_to_rename

def update_references_in_file(filepath, old_patterns, new_patterns):
    """Update all references to renamed files in a single file."""
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            content = f.read()
    except Exception:
        return False

    modified = False
    for old_pat, new_pat in zip(old_patterns, new_patterns):
        if old_pat in content:
            content = content.replace(old_pat, new_pat)
            modified = True

    if modified:
        try:
            with open(filepath, 'w', encoding='utf-8') as f:
                f.write(content)
            return True
        except Exception:
            return False

    return False

def update_all_references(files_to_rename, root_dir):
    """Update all references in the codebase."""
    # Build mapping of old → new names (with full paths)
    reference_map = {}
    for item in files_to_rename:
        old_dir = item['dir']
        old_name = item['old_name']
        new_name = item['new_name']

        # Store both just the filename and common path patterns
        reference_map[old_name] = new_name

        # Also store relative paths from docs/
        if '/docs/' in old_dir:
            old_rel = old_dir.replace(root_dir + '/docs/', '') + '/' + old_name
            new_rel = old_dir.replace(root_dir + '/docs/', '') + '/' + new_name
            reference_map[old_rel] = new_rel

    # Find all files to update (not renamed files themselves)
    files_to_update = []
    for dirpath, dirnames, filenames in os.walk(root_dir):
        # Skip these directories
        if any(skip in dirpath for skip in ['node_modules', '.git', 'pypi', '.next']):
            continue

        for filename in filenames:
            if filename.endswith(('.md', '.go', '.ts', '.js', '.json')):
                filepath = os.path.join(dirpath, filename)
                # Don't update files we're renaming
                if not any(filepath == item['old_path'] for item in files_to_rename):
                    files_to_update.append(filepath)

    # Update references in all files
    updates_made = defaultdict(list)
    for filepath in files_to_update:
        for old_name, new_name in reference_map.items():
            if update_references_in_file(filepath, [old_name], [new_name]):
                updates_made[filepath].append((old_name, new_name))

    return updates_made

def main():
    root_dir = '/Users/brenn/dev/gasoline'

    print("🔍 Scanning for files to rename...")
    files_to_rename = find_all_markdown_files(root_dir)

    if not files_to_rename:
        print("✅ No files need renaming!")
        return

    print(f"\n📋 Found {len(files_to_rename)} files to rename:")

    # Group by directory for clarity
    by_dir = defaultdict(list)
    for item in files_to_rename:
        dir_short = item['dir'].replace(root_dir + '/', '')
        by_dir[dir_short].append(item)

    for dir_name in sorted(by_dir.keys()):
        print(f"\n  {dir_name}:")
        for item in by_dir[dir_name]:
            print(f"    {item['old_name']} → {item['new_name']}")

    # Confirm action (auto-confirm in this context)
    print(f"\n⚠️  This will rename {len(files_to_rename)} files and update all references.")

    # Rename files
    print("\n📝 Renaming files...")
    renamed_count = 0
    for item in files_to_rename:
        old_path = item['old_path']
        new_path = os.path.join(item['dir'], item['new_name'])

        try:
            os.rename(old_path, new_path)
            renamed_count += 1
            print(f"  ✓ {item['old_name']} → {item['new_name']}")
        except Exception as e:
            print(f"  ✗ Failed to rename {item['old_name']}: {e}")

    print(f"\n✅ Renamed {renamed_count} files")

    # Update references
    print("\n🔗 Updating references in codebase...")
    updates = update_all_references(files_to_rename, root_dir)

    print(f"✅ Updated references in {len(updates)} files")
    for filepath, changes in sorted(updates.items()):
        filepath_short = filepath.replace(root_dir + '/', '')
        print(f"  {filepath_short}: {len(changes)} reference(s)")

    print("\n✨ Standardization complete!")
    print("\nNext steps:")
    print("1. python3 scripts/lint-documentation.py")
    print("2. python3 scripts/generate-feature-navigation.py")
    print("3. git add -A && git commit -m 'docs: Standardize all filenames to lowercase-with-hyphens'")

if __name__ == '__main__':
    main()
