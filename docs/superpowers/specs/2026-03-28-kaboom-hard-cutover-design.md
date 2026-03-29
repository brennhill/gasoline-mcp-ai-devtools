# Kaboom Hard-Cutover Design

## Goal

Replace the current mixed `Kaboom` / `Kaboom` identity with one new canonical identity:

- product name: `Kaboom`
- site/domain identity: `gokaboom.dev`
- primary visual mark: the original flame icon

This is a true hard cutover. The repo should not preserve runtime compatibility aliases, mixed-brand wording, or migration shims for the old names.

## Decision Summary

The approved direction is:

- hard cutover, not phased public compatibility
- full rename, not just user-facing copy
- wipe old state instead of migrating it
- aggressively remove first-party managed `Kaboom` and `Kaboom` artifacts during install, update, and uninstall
- restore the original flame icon as the canonical visual asset

## Problem Statement

The repo currently contains three overlapping brand layers:

- legacy `Kaboom`
- recent `Kaboom`
- a partially renamed site rooted in `gokaboom.dev` while older references still point to `gokaboom.com`

That mixed state creates product confusion, inconsistent documentation, conflicting install commands, and cleanup risk for customers who may have one or more legacy variants installed locally.

The new branding must eliminate the mixed state and make `Kaboom` the only supported identity inside the codebase and shipped artifacts.

## Constraints

- No compatibility aliases for old names in first-party runtime flows.
- No state migration from `Kaboom` or `Kaboom`.
- All first-party managed legacy assets should be removed during install/update/uninstall.
- The original flame icon should replace the Kaboom branding assets everywhere possible.
- The change should cover user-facing branding, packaging, runtime contracts, and installer behavior.
- The implementation must still satisfy the repo documentation cross-reference contract.

## Assumptions

- In-repo URLs, names, and packaging metadata will move to Kaboom naming even if external infrastructure such as domains, GitHub repo slugs, or package registry names still need separate operational follow-through.
- The target naming map is:
  - `Kaboom` -> `Kaboom`
  - `Kaboom` -> `Kaboom`
  - `gokaboom.com` -> `gokaboom.dev`
  - `gokaboom.dev` -> `gokaboom.dev`
  - `kaboom-mcp` -> `kaboom-mcp`
  - `kaboom-agentic-browser` -> `kaboom-agentic-browser`
  - repo/import/package families should move to Kaboom-derived names consistently
- Any customer-owned content outside installer-managed locations is not deleted automatically.

## Non-Goals

- Preserving backward compatibility for old commands or runtime identifiers.
- Migrating saved settings, tracked tabs, sessions, logs, caches, or local state from old brands into Kaboom.
- Supporting mixed-brand coexistence after the cutover.
- Designing new product features as part of the rename.

## Canonical Identity

### Product

- Display name: `Kaboom`
- Terminal/panel/product references: `Kaboom`
- All user-facing titles, tooltips, CTAs, setup instructions, and docs references use `Kaboom`

### Site

- Canonical site identity: `gokaboom.dev`
- Site source directory, site build paths, metadata, canonical URLs, and docs generators should use the new domain identity

### Iconography

- The original flame icon becomes the primary icon asset again
- Kaboom-specific visual marks are removed from the shipped surfaces
- Animated variants may remain only if they are based on the restored flame identity rather than the Kaboom mark

## Rename Surface

### 1. Extension And UI Surfaces

Update all extension-facing product identity:

- extension manifest name and description
- action title and popup labels
- side panel titles and fallback states
- hover launcher labels, tooltips, and settings copy
- options page text and diagnostics messages
- setup/install/uninstall guidance shown in extension surfaces
- icon assets and derived PNG outputs

### 2. Site And Docs Surfaces

Update all site-facing and docs-facing identity:

- site source root currently under `gokaboom.dev`
- canonical/meta URLs
- footer, CTA, hero, and install/download copy
- markdown mirror generators and llms text outputs
- docs references to old site names and product names
- flow maps, feature indexes, and standards docs that mention old names or domains

### 3. Packaging And Distribution

Update all first-party distribution identity:

- root package metadata
- npm package names and package directories
- PyPI package names and package directories
- binary names and wrapper scripts
- extension zip names and generated release artifact names
- install/update/uninstall copy in docs and setup UIs

### 4. Repository And Source Identity

Update source-level identity where represented in repo content:

- repository URLs in package metadata and docs
- source comments and human-readable file headers mentioning old names
- tests and fixtures that intentionally assert product names
- scripts that refer to old package or site paths
- site contract scripts and build commands tied to old site directory names

### 5. Runtime Contracts And Internal Identifiers

This cutover includes runtime-facing identifiers, not just copy:

- HTTP headers such as `X-Kaboom-*` become `X-Kaboom-*`
- resource URIs such as `kaboom://...` become `kaboom://...`
- extension storage keys prefixed with `kaboom_*` become `kaboom_*`
- env vars such as `KABOOM_*` or `Kaboom_*` become `KABOOM_*`
- developer API names such as `window.__kaboom` become `window.__kaboom`
- debug log prefixes and telemetry product labels become `Kaboom`

Old state is deleted rather than translated into the new keys.

## Cleanup Model

Kaboom install/update/uninstall should behave as cleanup operations, not passive add/remove flows.

### First-Party Managed Artifacts To Remove

The cleanup pass should aggressively delete known managed artifacts under the old brands:

- legacy binaries and hooks
- legacy unpacked extension directories
- legacy installer-managed extension handoff directories
- legacy managed MCP config blocks
- legacy installer-managed skills and helper assets
- legacy state directories, caches, logs, and temp data owned by the product
- legacy packaging wrappers or generated helper files under known managed locations

### What Not To Delete Blindly

Cleanup should remain precise for anything user-authored:

- arbitrary customer files outside managed paths
- unrelated MCP entries not owned by this product
- user-created skills that merely reference old names

The contract is aggressive cleanup for first-party owned assets, not indiscriminate filesystem deletion.

## Install, Update, And Uninstall Semantics

### Install

Install should run as:

1. detect legacy Kaboom/Kaboom/Kaboom managed assets
2. stop running daemons for old and new brands
3. remove first-party managed legacy assets and old state
4. remove managed skills and config entries for old brands
5. install fresh Kaboom assets only

Result:

- no old-brand binaries or managed config remain
- no old-brand state remains
- customer lands in a clean Kaboom install

### Update

Update should run as:

1. cleanup legacy and stale managed assets
2. cleanup stale Kaboom assets that would conflict
3. install the new Kaboom version

Update is effectively `wipe managed state + reinstall`, not an in-place preserve-and-patch flow.

### Uninstall

Uninstall should remove:

- Kaboom-managed assets
- Kaboom-managed assets
- Kaboom-managed assets
- managed skills, configs, caches, logs, and extension handoff artifacts across those names

Uninstall should leave no first-party managed residue behind.

## Architecture Impact

The cutover affects several architecture layers:

### Branding Layer

One shared source of truth should drive product name, site domain, and icon asset references so the repo does not drift into mixed branding again.

### Packaging Layer

Installer, package, and binary naming must be consistent across:

- root package metadata
- npm packages
- PyPI packages
- built binaries and release artifacts

### Runtime Layer

Runtime contracts must move in lockstep:

- headers
- storage keys
- resource URIs
- env vars
- window API names
- setup/health guidance strings

### Cleanup Layer

A dedicated cleanup path should own old-brand removal logic rather than scattering ad hoc delete logic across install/update/uninstall call sites.

## Implementation Waves

This remains one hard-cutover program, but execution should be structured in waves so the work is verifiable:

### Wave 1: Visual And User-Facing Branding

- restore flame icon
- remove Kaboom iconography
- rename extension/product/site copy to Kaboom
- rename visible site/domain references to `gokaboom.dev`

### Wave 2: Packaging And Site Structure

- rename site source paths and site scripts
- rename package names and directories
- rename binary and artifact names
- rename repo URLs and install/update/uninstall instructions

### Wave 3: Runtime Contracts

- rename headers, env vars, storage keys, resource URIs, and developer API names
- remove old-state reads and migration code
- ensure cleanup removes any legacy runtime state before fresh install

### Wave 4: Legacy Cleanup Hardening

- centralize legacy cleanup detection/removal
- ensure install/update/uninstall all call the same cleanup layer
- cover managed skills and managed config removal

## Error Handling

- Mixed-brand managed installs should be treated as invalid and cleaned before proceeding.
- Missing legacy artifacts during cleanup should not fail the operation; cleanup should be idempotent.
- Config rewrite should only remove known first-party managed entries, not unrelated customer entries.
- Uninstall should report partial cleanup failures clearly when managed artifacts cannot be removed.
- If external infrastructure still points to old domains or repo URLs, in-repo references should still point to the Kaboom targets and document the dependency clearly.

## Testing Strategy

The implementation should be planned with explicit regression coverage for:

- branding string changes in extension/site surfaces
- package/install metadata rename correctness
- cleanup behavior for legacy Kaboom/Kaboom assets
- install/update/uninstall deleting old managed artifacts
- state wipe behavior rather than migration behavior
- runtime contract rename correctness
- repo-wide legacy-name detection gates

Although QA execution happens after implementation, the implementation plan should require tests first for each behavior-replacing change.

## Documentation Contract

The implementation must update:

- canonical flow maps that describe installer/packaging/site/extension flows touched by the rename
- feature-local flow-map pointers where branding or packaging behavior changes
- feature `index.md` files with updated `last_reviewed`, `code_paths`, and `test_paths`
- `docs/architecture/flow-maps/README.md` if canonical map names or entries change
- docs references to old site roots, domains, package names, and runtime terminology

## Open Planning Risks

- Repo and package directory renames will have broad path fallout and need a careful plan before edits.
- Some generated or vendored metadata may need regeneration rather than direct manual edits.
- External systems such as live DNS, actual GitHub repo rename, package registry availability, and marketplace naming are operational dependencies outside pure code changes.

These are planning concerns, not reasons to soften the hard-cutover requirement.
