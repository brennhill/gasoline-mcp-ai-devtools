#!/usr/bin/env node

import { execSync } from 'node:child_process';
import { promises as fs } from 'node:fs';
import path from 'node:path';

const DOCS_PREFIX = 'cookwithgasoline.com/src/content/docs/';
const DOC_EXT_RE = /\.(md|mdx)$/;

function splitEnvFileList(value) {
  return value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function runGitDiff(range) {
  const output = execSync(`git diff --name-only --diff-filter=ACMR ${range}`, {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'ignore'],
  });

  return output
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean);
}

function getChangedFiles() {
  if (process.env.CONTENT_CONTRACT_FILES) {
    return splitEnvFileList(process.env.CONTENT_CONTRACT_FILES);
  }

  const changed = new Set();
  const ranges = [];

  if (process.env.CONTENT_CONTRACT_RANGE) {
    ranges.push(process.env.CONTENT_CONTRACT_RANGE);
  }

  if (process.env.GITHUB_BASE_REF) {
    ranges.push(`origin/${process.env.GITHUB_BASE_REF}...HEAD`);
  }

  ranges.push('HEAD~1..HEAD');

  for (const range of ranges) {
    try {
      const files = runGitDiff(range);
      for (const file of files) {
        changed.add(file);
      }
    } catch {
      // Try next range.
    }
  }

  try {
    const workingTree = execSync('git diff --name-only --diff-filter=ACMR', {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    })
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean);

    for (const file of workingTree) {
      changed.add(file);
    }
  } catch {
    // Ignore and return gathered set.
  }

  try {
    const untracked = execSync('git ls-files --others --exclude-standard', {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    })
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean);

    for (const file of untracked) {
      changed.add(file);
    }
  } catch {
    // Ignore and return gathered set.
  }

  return Array.from(changed);
}

function requireFrontmatterBlock(content) {
  const match = content.match(/^---\n([\s\S]*?)\n---\n?/);
  if (!match) {
    return null;
  }

  return {
    raw: match[1],
    body: content.slice(match[0].length),
  };
}

function hasFrontmatterKey(frontmatter, key) {
  return new RegExp(`^${key}:\\s*(.+)$`, 'm').test(frontmatter);
}

function hasHeading(body, headingLevel, headingText) {
  const escaped = headingText.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const pattern = new RegExp(`^${'#'.repeat(headingLevel)}\\s+${escaped}\\b`, 'm');
  return pattern.test(body);
}

function hasAnyH2(body) {
  return /^##\s+\S/m.test(body);
}

function validateFile(relativePath, content) {
  const errors = [];
  const frontmatter = requireFrontmatterBlock(content);

  if (!frontmatter) {
    return ['Missing frontmatter block (--- ... ---).'];
  }

  if (!hasFrontmatterKey(frontmatter.raw, 'title')) {
    errors.push('Missing frontmatter key: title');
  }

  if (!hasFrontmatterKey(frontmatter.raw, 'description')) {
    errors.push('Missing frontmatter key: description');
  }

  const isSplashTemplate = /^template:\s*splash\s*$/m.test(frontmatter.raw);

  if (!isSplashTemplate && !hasAnyH2(frontmatter.body)) {
    errors.push('Missing at least one H2 heading (## ...) in document body.');
  }

  if (relativePath.includes('/reference/')) {
    if (!hasHeading(frontmatter.body, 2, 'Quick Reference')) {
      errors.push('Reference page must include "## Quick Reference".');
    }

    if (!hasHeading(frontmatter.body, 2, 'Common Parameters')) {
      errors.push('Reference page must include "## Common Parameters".');
    }
  }

  if (relativePath.includes('/blog/')) {
    if (!hasFrontmatterKey(frontmatter.raw, 'date')) {
      errors.push('Blog post must include frontmatter key: date');
    }

    if (!hasFrontmatterKey(frontmatter.raw, 'authors')) {
      errors.push('Blog post must include frontmatter key: authors');
    }

    if (!hasFrontmatterKey(frontmatter.raw, 'tags')) {
      errors.push('Blog post must include frontmatter key: tags');
    }
  }

  return errors;
}

async function main() {
  const changed = getChangedFiles();
  const docsTargets = changed.filter((file) => file.startsWith(DOCS_PREFIX) && DOC_EXT_RE.test(file));

  if (docsTargets.length === 0) {
    console.log('Content contract: no changed docs/blog files detected.');
    return;
  }

  const violations = [];

  for (const relativePath of docsTargets) {
    const absolutePath = path.resolve(relativePath);
    const content = await fs.readFile(absolutePath, 'utf8');
    const fileErrors = validateFile(relativePath, content);

    if (fileErrors.length > 0) {
      violations.push({ file: relativePath, errors: fileErrors });
    }
  }

  if (violations.length === 0) {
    console.log(`Content contract: ${docsTargets.length} changed docs/blog file(s) passed.`);
    return;
  }

  console.error('Content contract violations found:\n');
  for (const violation of violations) {
    console.error(`- ${violation.file}`);
    for (const error of violation.errors) {
      console.error(`  - ${error}`);
    }
  }

  process.exit(1);
}

main().catch((error) => {
  console.error('Failed to run content contract check:', error);
  process.exit(1);
});
