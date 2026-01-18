# Quality Standards - Quick Reference Card

> **Print this and keep it visible while coding**

---

## ⚡ Before Every Commit

```bash
make quality-gate    # Runs all checks
```

Or run individually:
```bash
make check-file-length  # Files under 800 lines?
npm run lint            # 0 ESLint errors?
npm run typecheck       # TypeScript strict?
go vet ./...            # Go static analysis?
go test ./...           # All tests pass?
npm run test:ext        # TS tests pass?
```

---

## 🚫 **Never** Commit

❌ **Silent errors:** `_ = json.Unmarshal(...)` → **Check** the error!
❌ **Empty catches:** `.catch(() => {})` → **Log** the error!
❌ **TypeScript `any`:** Use proper types or `unknown`
❌ **Magic numbers:** Use named constants
❌ **Files > 800 lines:** Split into smaller modules
❌ **Unused imports/variables:** **Clean** them up
❌ **Missing tests:** Add tests for new code

---

## ✅ **Always** Do

✅ **Handle all errors** - **No** silent failures
✅ **Add context timeouts** - HTTP calls need `context.WithTimeout()`
✅ **Document `any` usage** - Add comment explaining why
✅ **Check file close** - On write operations
✅ **Use `safeMarshal()`** - For JSON marshal operations
✅ **Add tests** - Unit tests for new code
✅ **Update docs** - Keep doc.go current
✅ **Format TODOs** - Use `TODO(future):` with explanation

---

## 📏 Standards

| Item | Limit | Enforcement |
|------|-------|-------------|
| File length | 800 lines | `make check-file-length` |
| Function length | ~50 lines | Manual review |
| ESLint errors | 0 | `npm run lint` |
| TypeScript `any` | 0* | `npm run typecheck` |
| Test coverage | > 80% | Manual review |
| Benchmark regressions | < 20% | `go test -bench=.` |

*With justification comment

---

## 🔧 Error Handling Pattern

```go
// MCP tools
if err := json.Unmarshal(args, &params); err != nil {
    return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID,
        Result: mcpStructuredError(ErrInvalidJSON,
            "Invalid JSON arguments: "+err.Error(),
            "Fix JSON syntax and call again")}
}

// JSON marshal
return safeMarshal(result, `{"error":"marshal failed"}`)

// HTTP with context
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)

// File close on writes
defer func() {
    if closeErr := file.Close(); closeErr != nil {
        fmt.Fprintf(os.Stderr, "[gasoline] Error closing file: %v\n", closeErr)
    }
}()
```

```typescript
// TypeScript catch
.catch(err => console.error('[Gasoline] Error:', err))

// PostMessage origin
window.postMessage(msg, window.location.origin) // NOT '*'
```

---

## 🏗️ File Organization

**Go packages:**
```
cmd/dev-console/
  ├── main.go              # Entry point, flags
  ├── server.go            # Server struct
  ├── server_routes.go     # HTTP routes
  ├── tools_core.go        # ToolHandler
  ├── tools_observe.go     # Observe tool
  ├── tools_*.go           # Other tools
  └── *_test.go            # Tests
```

**New package checklist:**
- [ ] Create `doc.go` with package overview
- [ ] Add `*_test.go` with tests
- [ ] Add `*_bench_test.go` if performance-critical

---

## 🎯 Type Safety

**Go:**
```go
// ❌ Bad
func process(data interface{}) { ... }

// ✅ Good
func process[T any](data T) { ... }
// OR
// any: JSON schema requires dynamic types
func process(data any) { ... }
```

**TypeScript:**
```typescript
// ❌ Bad
function handle(msg: any) { ... }

// ✅ Good
interface Message { type: string; data: unknown }
function handle(msg: Message) { ... }
```

---

## 🔐 Security Checklist

- [ ] Validate all inputs
- [ ] Rate limit upload endpoints
- [ ] Use `window.location.origin` for PostMessage
- [ ] Add context timeouts to HTTP calls
- [ ] Check origins in CORS middleware
- [ ] Redact sensitive data from logs
- [ ] Use `crypto/rand` for security-sensitive IDs*

*math/rand is OK for non-security uses (correlation IDs, jitter)

---

## 📊 Performance Targets

| Operation | Target | Current |
|-----------|--------|---------|
| Ring buffer write | < 100 ns | 67 ns ✅ |
| WebSocket capture | < 100 µs | 123 µs ⚠️ |
| Network capture | < 50 µs | 27 µs ✅ |
| Action capture | < 50 µs | 14 µs ✅ |

Run benchmarks: `go test -bench=. ./...`

---

## 🐛 When You Find a Bug

1. **Write a failing test first**
2. **Fix the bug**
3. **Verify test passes**
4. **Add to regression suite**
5. **Document in commit message**

---

## 📝 Commit Message Format

```
type(scope): Short description

Detailed explanation of what and why.

Test Results:
- Go: All pass
- TypeScript: All pass
- Benchmarks: No regressions

Breaking changes: None

Co-Authored-By: Claude Sonnet 4.5 (1M context) <noreply@anthropic.com>
```

Types: `feat`, `fix`, `refactor`, `docs`, `test`, `perf`, `chore`

---

## 🚀 Quick Commands

```bash
# Development
make dev                # Build and run
make test-fast          # Quick tests
make quality-gate       # Full quality check

# Before PR
make ci-local           # Run full CI locally
make verify-all         # Comprehensive verification

# Debugging
go test -v ./pkg/...    # Verbose tests
go test -run TestName   # Run specific test
go test -bench=.        # Run benchmarks
```

---

## 🎓 Key Principles

1. **Simplicity** - Don't over-engineer
2. **Clarity** - Code should be obvious
3. **Safety** - Type-safe, error-checked
4. **Performance** - Benchmark hot paths
5. **Testability** - Everything tested
6. **Maintainability** - Files focused and small
7. **Security** - Defense in depth
8. **Documentation** - Explain the why

---

## 📞 When Stuck

1. Check [quality-standards.md](quality-standards.md) for detailed guidance
2. Review recent code reviews for patterns
3. Run `make quality-gate` to catch issues early
4. Ask for code review before committing large changes

---

**Updated:** 2026-02-02
**Version:** 5.4.0
**Target:** Top 1% Code Quality ✅
