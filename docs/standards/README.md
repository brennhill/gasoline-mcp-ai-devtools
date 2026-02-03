# Development Standards & Code Quality Guide

> **Comprehensive standards for implementing features at the top 1% quality level**

Welcome! This directory contains focused, cross-referenced standards docs that replace the original monolithic `feature-development-standards.md`. Each doc is optimized for specific use cases.

---

## 📚 Quick Navigation

### By Use Case (Start Here)

**Designing a data model?**
→ [data-design.md](data-design.md) — Naming, documentation, type safety, dependency injection

**Building an API endpoint?**
→ [api-and-security.md](api-and-security.md) — RESTful design, error handling, rate limiting, security

**Validating user input?**
→ [validation-patterns.md](validation-patterns.md) — Trust model, Gasoline-specific patterns (timestamps, URLs, correlation IDs)

**Handling errors and recovery?**
→ [error-and-recovery.md](error-and-recovery.md) — Error logging, graceful degradation, resource cleanup

**Optimizing performance?**
→ [memory-and-performance.md](memory-and-performance.md) — Complexity analysis, memory safety, 3-tier memory limits

**Writing clean, testable code?**
→ [code-quality.md](code-quality.md) — Readability, testing, concurrency, type safety, code organization

---

## 📖 All Documents

| Document | Size | Focus | When to Use |
|----------|------|-------|------------|
| [data-design.md](data-design.md) | ~300 lines | Data models, functions, DI | Designing types and APIs |
| [validation-patterns.md](validation-patterns.md) | ~300 lines | Input validation, trust model | Validating external inputs |
| [memory-and-performance.md](memory-and-performance.md) | ~400 lines | Memory safety, 3-tier limits, performance | Performance optimization |
| [error-and-recovery.md](error-and-recovery.md) | ~250 lines | Error handling, recovery, cleanup | Designing error paths |
| [api-and-security.md](api-and-security.md) | ~300 lines | HTTP APIs, security, rate limiting | Building endpoints |
| [code-quality.md](code-quality.md) | ~400 lines | Readability, testing, concurrency | Day-to-day implementation |

---

## 🔗 Cross-Reference Map

How the standards relate to each other:

```
┌─────────────────────────────────────────────────────┐
│   Data Design (types, functions, DI)                │
└──────────────────┬──────────────────────────────────┘
                   │
        ┌──────────┴─────────────┐
        │                        │
        ▼                        ▼
    ┌─────────────────┐   ┌──────────────────┐
    │  Validation     │   │  Code Quality    │
    │  Patterns       │   │  (testing, DI)   │
    └────────┬────────┘   └────────┬─────────┘
             │                     │
             │    ┌────────────────┘
             │    │
             ▼    ▼
        ┌──────────────────────┐
        │  Error & Recovery    │
        │  (logging, cleanup)  │
        └───────────┬──────────┘
                    │
        ┌───────────┴──────────┐
        │                      │
        ▼                      ▼
    ┌────────────────┐   ┌──────────────────┐
    │  Memory &      │   │  API & Security  │
    │  Performance   │   │  (HTTP, rate-    │
    │  (3-tier, GC) │   │   limit, CORS)   │
    └────────────────┘   └──────────────────┘
```

---

## 🎯 Feature Implementation Workflow

When implementing a new feature:

1. **Gate 1: Product Spec**
   - What are we building?
   - Use [data-design.md](data-design.md) for data model decisions

2. **Gate 2: Tech Spec**
   - How will we build it?
   - Cross-reference relevant standards:
     - [api-and-security.md](api-and-security.md) if building endpoints
     - [validation-patterns.md](validation-patterns.md) for input handling
     - [memory-and-performance.md](memory-and-performance.md) for performance budgets

3. **Gate 3: Implementation**
   - Follow [code-quality.md](code-quality.md) for daily coding standards
   - Use [error-and-recovery.md](error-and-recovery.md) for error handling

4. **Gate 4: Testing**
   - Reference testing standards in [code-quality.md](code-quality.md)
   - Ensure 90%+ coverage

5. **Gate 5: Review**
   - Check against [api-and-security.md](api-and-security.md) for security
   - Verify [memory-and-performance.md](memory-and-performance.md) budgets met

---

## 📋 Daily Use Checklists

### Before Starting a Feature
- [ ] Read [data-design.md](data-design.md) section on naming and design
- [ ] Identify which other standards are relevant
- [ ] Reference them in tech spec

### During Implementation
- [ ] Follow [code-quality.md](code-quality.md) for code organization
- [ ] Use [error-and-recovery.md](error-and-recovery.md) for error paths
- [ ] Apply [validation-patterns.md](validation-patterns.md) at boundaries
- [ ] Test with [code-quality.md](code-quality.md) strategies

### Before Code Review
- [ ] Run `make quality-gate`
- [ ] Check against [memory-and-performance.md](memory-and-performance.md)
- [ ] Verify [api-and-security.md](api-and-security.md) security checks
- [ ] Ensure all files under 800 lines per [code-quality.md](code-quality.md)

---

## 🔑 Key Principles Summary

### Across All Standards

1. **Semantic naming** - Domain language, not abbreviations
2. **Document intent** - Explain WHY, not just WHAT
3. **Type safety** - No `any`, strict TypeScript, concrete Go types
4. **Validation early** - At system boundaries, whitelist approach
5. **Error handling** - ALL operations logged, no silent failures
6. **Resource cleanup** - Always defer, LIFO order
7. **Performance budgets** - Define, measure, track
8. **Test first** - TDD, 90%+ coverage
9. **Concurrency-safe** - Race detector, mutex discipline
10. **Zero dependencies** - Gasoline rule for production

---

## 🛠️ Gasoline-Specific Patterns

These standards incorporate Gasoline-specific patterns you'll see repeatedly:

### Memory Management
- **3-tier limits:** Soft (50MB), Hard (100MB), Critical (150MB)
- **Ring buffers:** FIFO eviction, capacity limits
- **String truncation:** Max 10KB per entry

### Validation
- **Timestamps:** RFC3339/RFC3339Nano formats
- **URLs:** SSRF prevention, localhost validation
- **Correlation IDs:** Format: `prefix_timestamp_random`
- **Ranges:** Validate numeric bounds early

### Concurrency
- **Thread-safe by default:** Document if not safe
- **Mutex discipline:** defer unlock, minimize scope
- **Context everywhere:** Goroutines accept context

---

## ⚡ Common Patterns Index

Need to implement something? Here's where to look:

| Need | See |
|------|-----|
| New data type | [data-design.md](data-design.md) → Naming + Documentation |
| New function | [data-design.md](data-design.md) → Functions & Methods |
| HTTP endpoint | [api-and-security.md](api-and-security.md) → API Design |
| Input validation | [validation-patterns.md](validation-patterns.md) → Gasoline patterns |
| Error handling | [error-and-recovery.md](error-and-recovery.md) → Error Logging |
| Memory optimization | [memory-and-performance.md](memory-and-performance.md) → Memory Safety |
| Test suite | [code-quality.md](code-quality.md) → Testing |
| Concurrent code | [code-quality.md](code-quality.md) → Concurrency |
| Code organization | [code-quality.md](code-quality.md) → Code Organization |
| Security review | [api-and-security.md](api-and-security.md) → Security |
| Rate limiting | [api-and-security.md](api-and-security.md) → Rate Limiting |
| Resource cleanup | [error-and-recovery.md](error-and-recovery.md) → Resource Management |

---

## 📚 Related Documentation

- [.claude/docs/feature-workflow.md](../../.claude/docs/feature-workflow.md) — 5-gate process
- [docs/examples/feature-request-template.md](../examples/feature-request-template.md) — Feature request format
- [docs/quality-standards.md](../quality-standards.md) — Quality maintenance guide
- [docs/quality-quick-reference.md](../quality-quick-reference.md) — Quick quality checklist
- [docs/README.md](../README.md) — Master documentation index

---

## 🔄 Evolution & Updates

- **Last updated:** 2026-02-03
- **Source:** Split from 1380-line monolithic document
- **Coverage:** 100% of original content preserved
- **Size:** All files under 500 lines per document
- **Status:** Active - evolving with project needs

To suggest improvements, file an issue or PR referencing the specific standards document.

---

## 📞 Questions?

- Not sure which doc to start with? → Begin with **Quick Navigation** above
- Looking for a specific pattern? → Check **Common Patterns Index**
- Want to understand relationships? → See **Cross-Reference Map**

**Remember:** These standards are foundations for top 1% quality. Apply them consistently, and update docs when standards evolve.

