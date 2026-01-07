## Template: Known Issue Entry

Use this format when adding entries to `/KNOWN-ISSUES.md`:

```markdown
### Short description of the issue
- **Status:** Open | Fix in progress | Fixed in vX.Y.Z
- **Severity:** Critical | High | Medium | Low
- **Version affected:** X.Y.Z - X.Y.Z
- **Symptom:** What the user or LLM agent observes
- **Workaround:** Temporary fix (if available)
- **Root cause:** Technical explanation (if known)
- **Tracking:** Link to spec or in-progress doc
- **Fix ETA:** Target version (e.g., v5.2)
```

### Severity Definitions

| Severity | Definition |
|----------|-----------|
| Critical | Data loss, security vulnerability, or complete feature failure |
| High | Core feature broken, no workaround |
| Medium | Feature partially broken, workaround available |
| Low | Cosmetic, minor inconvenience, edge case |
