# BlazeTorch AI Devstack — Rebrand Master Plan

**From:** Gasoline Agentic Browser Devtools MCP
**To:** BlazeTorch AI Devstack
**Status:** Planning (not yet executed)

---

## Phase 1: Pre-Rebrand (Planning & Setup)

### 1.1 Domain & Infrastructure
- [ ] Register **blazetorch.dev** (already owned)
- [ ] Setup DNS for blazetorch.dev (where does it point?)
- [ ] Verify cookwithgasoline.com redirect plan
- [ ] Decide: Keep cookwithgasoline.com live or redirect 301?
- [ ] Setup SSL/HTTPS for blazetorch.dev

### 1.2 GitHub Strategy
**Decision point:** Rename repo or keep current name?

**Option A: Hard rename**
- Rename: `gasoline-agentic-browser-devtools-mcp` → `blazetorch-ai-devstack`
- Rename org (if applicable)
- 301 redirects on old URLs (GitHub handles this)
- Old GitHub URLs break until migration completes

**Option B: Soft rebrand**
- Keep repo name as-is (avoid disruption)
- Update descriptions, README, docs
- GitHub org name stays same
- Less disruptive, but confusing naming long-term

**Recommendation:** Clarify with user before proceeding

### 1.3 Package Names Strategy
**NPM/distributions:**
- Current: `gasoline-mcp` (if published)
- New: `blazetorch-mcp` or `blazetorch-ai-devstack`?
- Extension: Still "Gasoline" in Chrome Web Store?
- Binary: Still `gasoline` CLI command?

**Decision point:** Rename everything, or keep binary/CLI name for compatibility?

---

## Phase 2: Documentation & Content Updates

### 2.1 Website Migration (cookwithgasoline.com → blazetorch.dev)
**Astro site files to update:**

```
/cookwithgasoline.com/
├── astro.config.mjs
│   └── Update site URL from cookwithgasoline.com → blazetorch.dev
├── src/
│   ├── content/docs/ (all markdown files)
│   │   └── Search & replace: "Gasoline" → "BlazeTorch AI Devstack"
│   │   └── Search & replace: "gasoline" → "blazetorch"
│   ├── components/
│   │   ├── Header.astro - update branding/colors?
│   │   ├── Footer.astro - update copyright/links
│   │   └── Landing.astro - update hero copy, feature descriptions
│   └── assets/ - rebrand images/logos?
└── public/ - update favicon, OG images
```

**Content to update:**
- [ ] README.md (all branches)
- [ ] All docs (features, guides, reference)
- [ ] Blog posts (update references)
- [ ] CHANGELOG.md
- [ ] Install scripts (URLs, branding)
- [ ] MCP setup guides

### 2.2 Code Repository Updates

**Files to update across codebase:**

```
/gasoline/ (main repo)
├── README.md
├── CHANGELOG.md
├── scripts/
│   ├── install.sh - update download URLs, branding
│   └── install.ps1 - update download URLs, branding
├── extension/ - extension branding
│   ├── manifest.json - update extension name
│   └── src/ - update console messages, user-facing text
├── Go binaries - update version strings, branding
├── docs/
│   └── All documentation files
└── .github/ - update workflows, templates
```

**Specific changes:**
- [ ] File headers (change "Gasoline" → "BlazeTorch")
- [ ] Package names (if renaming)
- [ ] Download URLs (point to blazetorch.dev)
- [ ] GitHub URLs (if repo renamed)
- [ ] Logo/branding in assets
- [ ] Extension name in Chrome Web Store

### 2.3 Search & Replace Scope

**Across all branches (STABLE, UNSTABLE, site):**

1. **"Gasoline" → "BlazeTorch"** (brand name)
2. **"gasoline" → "blazetorch"** (lowercase, URLs, CLI)
3. **"cookwithgasoline.com" → "blazetorch.dev"** (URLs)
4. **"Agentic Devtools" → "AI Devstack"** (tagline)

**Exclude:**
- git commit history (don't rewrite, too disruptive)
- Image assets (unless rebranding)
- Test fixtures/mocks (unless critical)

---

## Phase 3: Release & Migration

### 3.1 Timing Decision
- **Option A: Big Bang** - Release v0.8.1 or v0.9.0 with full rebrand
  - Pros: Clean, simple, one-time
  - Cons: More disruptive, single release vector

- **Option B: Gradual** - Rebrand docs first, release code later
  - Pros: Less disruptive, can test migrations
  - Cons: Confusing period with mixed branding

**Recommendation:** Clarify with user

### 3.2 Release Checklist
- [ ] Bump version (document rebrand in release notes)
- [ ] Update all docs
- [ ] Update GitHub (rename if chosen)
- [ ] Deploy blazetorch.dev
- [ ] Setup 301 redirects from cookwithgasoline.com
- [ ] Update social media / README links
- [ ] Tag release on GitHub

### 3.3 Communications
- [ ] Release notes explaining rebrand (why, what changed)
- [ ] Blog post on blazetorch.dev (optional)
- [ ] Update GitHub org description
- [ ] Update NPM package description (if applicable)
- [ ] Pin announcement in README

---

## Phase 4: Post-Rebrand Maintenance

### 4.1 Redirect Strategy
**cookwithgasoline.com → blazetorch.dev:**
- [ ] 301 redirects for all docs URLs
- [ ] Umami analytics migration
- [ ] Google Search Console updates
- [ ] Maintain both sites for 6+ months for SEO safety

### 4.2 Cleanup (Later)
- [ ] Remove old domain after 6-12 months
- [ ] Archive old repository (if renaming)
- [ ] Update all broken links in the wild

---

## Decision Points (User Input Needed)

1. **GitHub repo name:**
   - A) Rename `gasoline-*` → `blazetorch-*` (hard break)
   - B) Keep current name, rebrand in docs only (soft break)

2. **Package names:**
   - A) Rename `gasoline-mcp` → `blazetorch-mcp` (breaking change)
   - B) Keep current names for compatibility

3. **Extension/binary names:**
   - A) Rebrand everything (Chrome extension, CLI, etc.)
   - B) Keep backward-compatible names

4. **Release timing:**
   - A) Big bang release (v0.9.0 with rebrand)
   - B) Gradual rollout (docs first, code later)

5. **Old domain:**
   - A) 301 redirect + 6-month archive
   - B) Keep both sites running indefinitely

---

## Execution Order (Recommended)

1. **Make decisions above** (critical blockers)
2. **Setup blazetorch.dev** (DNS, infrastructure)
3. **Update website code** (astro.config.mjs, content, branding)
4. **Test site locally** (build, verify links)
5. **Update main repo** (README, docs, file headers)
6. **Create release branch** (with all changes)
7. **Test install scripts** (download URLs work)
8. **Deploy website** (blazetorch.dev live)
9. **Tag release** (v0.9.0 or v0.8.1)
10. **Setup redirects** (cookwithgasoline.com → blazetorch.dev)
11. **Announce** (release notes, blog, social)

---

## Risk Mitigation

- **Git history:** Don't rewrite (use git filter-repo was already done for vibe-annotations)
- **Backward compatibility:** Keep old package names if significant install base
- **Analytics:** Migrate Umami to new domain, keep old analytics for reference
- **SEO:** Use 301 redirects for 6+ months to preserve rankings
- **Testing:** Full UAT on blazetorch.dev before going live

---

## Questions for User

1. How disruptive is a GitHub rename? (impacts cloned repos, CI/CD)
2. How critical is backward compatibility for package names?
3. When can you commit time to testing/validation?
4. Any external integrations (CI/CD, package managers) that need updates?

