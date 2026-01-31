# Track This Tab - Specification Summary & Review

**Date:** 2026-01-27
**Status:** DRAFT - Ready for Review
**Priority:** CRITICAL (Security/Privacy Fix)

---

## Quick Links

- **Product Spec:** [spec-track-tab-product.md](./spec-track-tab-product.md)
- **Technical Spec:** [spec-track-tab-technical.md](./spec-track-tab-technical.md)
- **Edge Cases:** [spec-track-tab-edge-cases.md](./spec-track-tab-edge-cases.md)
- **Related Issues:** [tracking-analysis.md](./tracking-analysis.md)

---

## The Problem in One Sentence

**Current:** Extension captures telemetry from ALL browser tabs regardless of user intent.
**Fix:** Extension captures ONLY from the single tab the user explicitly tracks.

---

## Specification Overview

### 1. Product Specification (User Perspective)

**Key Changes:**
- Button renamed: "Track This Page" → "Track This Tab"
- Single-tab attachment model (explicit user control)
- "No Tracking" mode with clear LLM messaging
- Privacy guarantee: Other tabs never monitored

**User Workflows:**
- Debug single-page app in one tab
- Navigate multi-site flows within one tab
- Explicit enable/disable tracking
- Clear communication when tracking disabled

**Success Metrics:**
- Zero data captured from untracked tabs
- User controls what is tracked
- LLM can debug multi-site flows in one tab

📄 **Full Details:** [spec-track-tab-product.md](./spec-track-tab-product.md) (450 lines)

---

### 2. Technical Specification (Implementation)

**Architecture:**
- Filter messages in `content.js` before forwarding
- Check `trackedTabId` from storage per message
- Clear buffers on tracking switch
- Send status pings to server every 30s

**Files Changed:**
1. `extension/content.js` - Add tab filtering (~30 lines)
2. `extension/popup.js` - Update button text, block chrome:// URLs
3. `extension/popup.html` - Change button text
4. `extension/background.js` - Add status ping, clear buffers
5. `cmd/dev-console/status.go` - Handle status pings (NEW)
6. `cmd/dev-console/tools.go` - Check tracking before operations

**Implementation Complexity:**
- **Effort:** 2-3 hours
- **Risk:** Low (isolated change, no API surface changes)
- **Performance:** Negligible (< 0.01% overhead)

📄 **Full Details:** [spec-track-tab-technical.md](./spec-track-tab-technical.md) (850 lines)

---

### 3. Edge Case Analysis (Robustness)

**Analyzed 20 Edge Cases:**
- P0 (Must Handle): 6 cases - Tracked tab closed, tracking switched, extension reload, etc.
- P1 (Should Handle): 8 cases - Multiple windows, tab crashes, chrome:// pages, etc.
- P2 (Nice to Have): 6 cases - Storage cleared, no tabs, rapid toggle, incognito, etc.

**Most Critical:**
1. ✅ Tracked tab closed → Clear tracking, enter "No Tracking" mode (HANDLED)
2. ⚠️ Tracking switched → Clear buffers to prevent data mixing (NEEDS IMPL)
3. ⚠️ Content script not loaded → Detect and return clear error (NEEDS IMPL)
4. ⚠️ Tab crashes → Detect unresponsive, suggest refresh (NEEDS IMPL)
5. ⚠️ Chrome internal pages → Block tracking, disable button (NEEDS IMPL)

**Testing Priority:**
- P0 tests: 6 scenarios - Must pass before release
- P1 tests: 8 scenarios - Should pass before release
- P2 tests: 6 scenarios - Nice to have

📄 **Full Details:** [spec-track-tab-edge-cases.md](./spec-track-tab-edge-cases.md) (650 lines)

---

## Review Checklist

### Product Review

**User Experience:**
- [ ] Button text change is clear ("Track This Tab" vs "Track This Page")
- [ ] "No Tracking" mode messaging is helpful to LLMs
- [ ] User workflows cover common debugging scenarios
- [ ] Privacy guarantees are clearly stated
- [ ] Edge cases don't create confusing UX

**Open Questions:**
1. **Auto-switch on tab close?**
   - Spec recommends: Return to "No Tracking" mode (safer)
   - Alternative: Suggest switching to active tab
   - **Decision:** _______________

2. **Visual indicator on tracked tab?**
   - Extension badge icon showing "📍" when tracked?
   - **Decision:** _______________

3. **Incognito mode?**
   - Spec recommends: Block tracking (privacy)
   - Alternative: Allow with warning
   - **Decision:** _______________

4. **Allow tracking file:// URLs?**
   - Useful for local dev, needs permission
   - **Decision:** _______________

**Approval:** ❌ / ✅

**Notes:**
```


```

---

### Technical Review

**Architecture:**
- [ ] Filtering in content.js is the right layer
- [ ] Storage access overhead is acceptable
- [ ] Status ping every 30s is reasonable
- [ ] Buffer clearing won't cause data loss issues
- [ ] Error messages are actionable for LLMs

**Implementation:**
- [ ] Code examples are correct and complete
- [ ] Edge case handling is comprehensive
- [ ] Testing strategy covers critical paths
- [ ] Performance impact is negligible
- [ ] Rollback plan is feasible

**Security:**
- [ ] Tab ID spoofing not possible (Chrome APIs trusted)
- [ ] Content script isolation prevents page interference
- [ ] No new permissions required
- [ ] Privacy guarantees are technically sound

**Approval:** ❌ / ✅

**Notes:**
```


```

---

### Edge Case Review

**Coverage:**
- [ ] All edge cases identified are realistic
- [ ] Priority levels (P0/P1/P2) are correct
- [ ] P0 edge cases are all handled or have clear impl plan
- [ ] P1 edge cases are acceptable to defer if needed
- [ ] P2 edge cases are truly optional

**Testing:**
- [ ] Test scenarios are comprehensive
- [ ] Test scenarios are automatable (where possible)
- [ ] Manual test procedures are clear
- [ ] UAT checklist will be updated

**Missing Edge Cases?**
List any edge cases not covered:
```


```

**Approval:** ❌ / ✅

**Notes:**
```


```

---

## Implementation Approval

### Pre-Implementation Checklist

Before writing code:
- [ ] Product spec approved (user workflows, UX, privacy model)
- [ ] Technical spec approved (architecture, files changed, implementation)
- [ ] Edge cases approved (coverage, priority, handling)
- [ ] Open questions resolved (auto-switch, visual indicator, incognito)
- [ ] Testing strategy agreed upon
- [ ] Rollout plan confirmed (phase 1 critical fix only)

### Risks Acknowledged

- [ ] **Breaking change:** Users who relied on multi-tab capture will see behavior change
- [ ] **UX change:** Button text and behavior changes (minor confusion possible)
- [ ] **Implementation time:** 2-3 hours estimated (could be more if edge cases complex)
- [ ] **Testing time:** Manual UAT required (multi-tab scenarios hard to automate)

### Rollback Plan

If critical bug found after release:
1. Revert commit: `git revert <commit-hash>`
2. Rebuild extension: `make dev`
3. Reload extension
4. Time to rollback: < 5 minutes

### Go/No-Go Decision

**Ready to implement?**

- [ ] **GO** - All specs approved, start implementation
- [ ] **NO-GO** - Issues found, revise specs

**Blocker issues:**
```


```

**Sign-off:**
- Product Owner: _________________ Date: _______
- Technical Lead: ________________ Date: _______
- Security Review: _______________ Date: _______

---

## Post-Review: Next Steps

### If Approved (GO)

1. **Create implementation branch**
   ```bash
   git checkout -b fix/track-tab-isolation
   ```

2. **Implement in order:**
   - Phase 1: Core filtering (content.js) - 1 hour
   - Phase 2: UI updates (popup.js/html) - 30 min
   - Phase 3: Server status ping - 30 min
   - Phase 4: Edge case handling - 1 hour
   - Phase 5: Testing - 2 hours

3. **Test before commit:**
   - Run unit tests: `node --test tests/extension/*.test.js`
   - Run manual UAT: [UAT-TEST-PLAN.md](./UAT-TEST-PLAN.md)
   - Verify all P0 edge cases

4. **Update documentation:**
   - README.md (tracking model)
   - CHANGELOG.md (breaking change)
   - docs/architecture.md (data flow)

5. **Commit with reference:**
   ```bash
   git add .
   git commit -m "fix: implement single-tab tracking isolation

   BREAKING CHANGE: Extension now only captures from explicitly tracked tab.

   - Add tab filtering in content.js
   - Rename button 'Track This Page' → 'Track This Tab'
   - Add 'No Tracking' mode with LLM messaging
   - Handle edge cases (tab closed, tracking switch, etc.)

   Fixes critical security issue where all tabs captured data.

   See docs/core/SPEC-TRACK-TAB-*.md for full specification."
   ```

6. **Create PR for review**

### If Not Approved (NO-GO)

1. **Address blocker issues:**
   - Update specs based on feedback
   - Resolve open questions
   - Add missing edge cases

2. **Re-submit for review**

3. **Iterate until approved**

---

## Specification Quality Checklist

### Completeness
- [x] Product spec covers all user-facing behavior
- [x] Technical spec has implementation details with code
- [x] Edge cases analyzed and prioritized
- [x] Testing strategy defined
- [x] Documentation updates identified
- [x] Rollback plan provided

### Clarity
- [x] Specifications are unambiguous
- [x] Code examples are complete and correct
- [x] Edge cases have clear expected behavior
- [x] Review checklist guides reviewers
- [x] Open questions are explicitly called out

### Feasibility
- [x] Implementation effort estimated (2-3 hours)
- [x] No new dependencies required
- [x] No breaking API changes (only behavior change)
- [x] Rollback is straightforward
- [x] Testing is achievable

---

## Summary of Changes

### User-Visible Changes
1. Button text: "Track This Page" → "Track This Tab"
2. Button blocks chrome:// URLs
3. Popup shows "No tab tracked" when disabled
4. Only tracked tab sends data (privacy fix)
5. LLM gets clear error when tracking disabled

### Technical Changes
1. content.js: Add tab filtering (~30 lines)
2. popup.js: Update button, block internal URLs (~50 lines)
3. background.js: Status ping, buffer clearing (~50 lines)
4. Server: New /api/extension-status endpoint (~100 lines)
5. Server: Check tracking in all tools (~30 lines)

### Files Modified
- `extension/content.js`
- `extension/popup.js`
- `extension/popup.html`
- `extension/background.js`
- `cmd/dev-console/status.go` (NEW)
- `cmd/dev-console/tools.go`
- `README.md`
- `CHANGELOG.md`

**Total New Code:** ~260 lines
**Estimated Effort:** 2-3 hours implementation + 2 hours testing

---

## Questions for Reviewer

1. **Is the product spec clear about user workflows?**
   - Debug single-page app
   - Navigate multi-site flows
   - "No Tracking" mode

2. **Is the technical implementation sound?**
   - Filter in content.js (right layer?)
   - Status ping every 30s (too frequent?)
   - Clear buffers on switch (data loss OK?)

3. **Are edge cases handled sufficiently?**
   - P0 edge cases critical enough?
   - P1 edge cases can be deferred?
   - Missing any important scenarios?

4. **Are open questions resolved?**
   - Auto-switch on tab close? (Spec says no)
   - Visual indicator? (Spec says defer)
   - Incognito mode? (Spec says block)
   - file:// URLs? (Spec says allow if permitted)

5. **Is the testing strategy adequate?**
   - 6 P0 tests before release
   - Manual UAT procedures
   - Integration tests for isolation

---

## Conclusion

This specification defines a critical security fix that changes Gasoline from "captures all tabs" to "captures only tracked tab". The fix is:

- **Necessary:** Closes security/privacy vulnerability
- **Feasible:** 2-3 hours implementation, low risk
- **Well-specified:** 1950 lines of product/technical/edge case specs
- **Testable:** Clear UAT scenarios, automatable tests

**Recommendation:** Approve and proceed with implementation.

---

**Review Date:** ______________

**Approved By:** ______________

**Start Implementation:** ❌ / ✅
