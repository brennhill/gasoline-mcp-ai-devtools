---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Codacy Integration Setup

## GitHub Secrets Configuration

To enable Codacy scanning in CI/CD, add the following secret to your GitHub repository:

### Steps:
1. Go to: `Settings` → `Secrets and variables` → `Actions`
2. Click `New repository secret`
3. Add the following secret:
   - **Name:** `CODACY_API_TOKEN`
   - **Value:** `lHYOvUqzdGUcjC9p7wru`

Once added, the Codacy security scan will run automatically on:
- Push to `main` and `UNSTABLE` branches
- Pull requests to `main`
- Daily scheduled runs (6 AM UTC)

## Local Development Setup

To run Codacy scans locally:

### 1. Create local environment file:
```bash
cp .env.example .env
```

### 2. Update `.env` with your values:
```bash
export CODACY_API_TOKEN=lHYOvUqzdGUcjC9p7wru
export CODACY_ORGANIZATION_PROVIDER=gh
export CODACY_USERNAME=brennhill
export CODACY_PROJECT_NAME=gasoline-mcp-ai-devtools
```

### 3. Source the environment:
```bash
source .env
```

### 4. Run Codacy scan:
```bash
# If using Codacy CLI (install separately)
codacy-cli analyze
```

## GitHub Actions Workflow

The CI pipeline at `.github/workflows/ci.yml` includes a Codacy security scan step that:
- Runs when `CODACY_API_TOKEN` secret is configured
- Fails gracefully if token is not set
- Respects GitHub event context (skips for external forks)

## Security Notes

- ⚠️ **Never commit `.env` file** to the repository
- The token is only used in GitHub Actions (stored as a secret)
- For local development, the token stays in your local `.env` file (gitignored)
- Token is read-only for Codacy security scanning

## Troubleshooting

If scans aren't running:
1. Verify the secret was added to GitHub Settings
2. Check that the workflow has access to the secret
3. Review GitHub Actions logs for errors
4. Ensure branch is `main` or `UNSTABLE` (or triggering event is `workflow_dispatch`)
