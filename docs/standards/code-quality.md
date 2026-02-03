# Code Quality Standards

> **Readability, testing, concurrency, type safety, and code organization**

**Scope:** Writing readable, maintainable, testable code with strong type safety and proper concurrency handling.

**Related Standards:**
- [data-design.md](data-design.md) — Data structure design
- [memory-and-performance.md](memory-and-performance.md) — Performance considerations
- [error-and-recovery.md](error-and-recovery.md) — Error handling

---

## 🧵 Concurrency & Threading

### Goroutine Management

- ✅ **All goroutines accept context:** For clean shutdown
  ```go
  for {
      select {
      case <-ticker.C:
          doWork()
      case <-ctx.Done():
          return // Clean shutdown
      }
  }
  ```

- ✅ **Stop on context cancellation:** Graceful shutdown
- ✅ **Don't leak goroutines:** Every goroutine should have exit condition
- ✅ **Document goroutine lifecycle:** When started, when stopped

### Mutex Usage

- ✅ **Always defer unlock:** Ensures unlock even on panic
  ```go
  mu.Lock()
  defer mu.Unlock()
  ```

- ✅ **Minimize lock scope:** Don't hold locks during I/O
- ✅ **Document locking strategy:** Which mutex protects which data
- ✅ **Avoid deadlocks:** Consistent lock ordering, don't nest locks

### Race Condition Prevention

- ✅ **Run with race detector:** `go test -race ./...`
- ✅ **Parallel arrays need defensive checks:** Verify lengths match
- ✅ **Shared state needs protection:** Mutex or channels
- ✅ **Document thread safety:** Comment if function is thread-safe or not

---

## 🧪 Testing

### Test Naming

- ✅ **Descriptive names:** `TestAddNetworkBodies_ValidInput` not `TestAdd`
- ✅ **Table-driven for multiple cases:**
  ```go
  tests := []struct{
      name string
      input NetworkBody
      want error
  }{
      {"valid GET request", validBody, nil},
      {"empty URL", emptyURLBody, ErrInvalidURL},
  }
  ```

### Test Organization

- ✅ **Co-locate with code:** `network.go` → `network_test.go`
- ✅ **Separate unit and integration:** Use build tags for integration tests
- ✅ **Group related tests:** Use subtests `t.Run("subtest", ...)`

### Test Quality

- ✅ **Test happy path AND error paths**
- ✅ **Test edge cases:** Empty input, nil, max values, overflow
- ✅ **Test concurrent access:** If function is thread-safe, test it
- ✅ **Mock external dependencies:** Don't call real APIs in tests
- ✅ **Coverage target: 90%+** for new code

### Test Behavior

- ✅ **Tests are deterministic:** No flaky tests, no race conditions
- ✅ **Tests clean up:** Delete temp files, close connections
- ✅ **Tests are fast:** Unit tests < 100ms each
- ✅ **Tests are isolated:** One test failure doesn't affect others

---

## 🔍 Type Safety

### Go Type Safety

- ✅ **Prefer concrete types** over `any`
- ✅ **Use generics** for type-safe collections (Go 1.18+)
- ✅ **Document `any` usage:** Every `any` needs comment explaining why
- ✅ **Use type aliases** for clarity: `type UserID string`

### TypeScript Type Safety

- ✅ **ZERO `any`:** Use `unknown` and narrow with type guards
- ✅ **Strict mode enabled:** tsconfig.json has strict: true
- ✅ **Define interfaces:** For all data structures
- ✅ **Use generics:** For reusable components

### Type Assertions

- ✅ **Avoid type assertions** unless necessary
- ✅ **Check assertions:** Use type guards, handle failure
- ✅ **Document why:** If assertion is needed, explain

---

## 🏗️ Code Organization

### File Structure

- ✅ **Files under 800 lines:** Split if larger
- ✅ **One concern per file:** Don't mix HTTP handlers with business logic
- ✅ **Group related functionality:** All WebSocket code together
- ✅ **Consistent file naming:** `network_capture.go` not `NetCap.go`

### Package Boundaries

- ✅ **Clear package purpose:** Each package has one responsibility
- ✅ **Minimize exports:** Only export what other packages need
- ✅ **No circular dependencies:** Use interfaces to break cycles
- ✅ **Package documentation:** Every package has doc.go

### Import Organization

- ✅ **Group imports:** stdlib, external, internal
- ✅ **Remove unused imports:** Cleaned by goimports
- ✅ **Avoid dot imports:** No `import . "package"`

---

## 📖 Code Readability (Detailed Standards)

### Variable Naming (Context-Specific)

- ✅ **Use domain language:** `correlationID` not `corrId`, `networkBody` not `netBody`
- ✅ **Boolean names are questions:** `isEnabled`, `hasData`, `shouldRetry`
- ✅ **Collections are plural:** `events` not `event`, `bodies` not `body`
- ✅ **Receivers are short but clear:**
  ```go
  func (c *Capture) AddEvents()  // c for Capture
  func (rb *RingBuffer) Write()  // rb for RingBuffer
  func (h *ToolHandler) Handle() // h for ToolHandler
  ```

### Naming Anti-Patterns to Avoid

```go
// ❌ Bad - Abbreviations
func procReq(req *Req) (*Resp, err)

// ✅ Good - Full names
func processRequest(req *Request) (*Response, error)

// ❌ Bad - Unclear names
func handle(data interface{})

// ✅ Good - Specific names
func handleNetworkBody(body NetworkBody)

// ❌ Bad - Generic names
func get() interface{}

// ✅ Good - Describes what it gets
func getPendingQueries() []Query
```

### Function Length & Complexity

- ✅ **Target: < 30 lines** (ideal), max 50 lines
- ✅ **One level of abstraction:** Don't mix high-level and low-level operations
- ✅ **Extract helper functions:** If function does A then B then C, extract B and C
- ✅ **Early returns for guard clauses:**
  ```go
  func process(data []byte) error {
      // Guard clauses first (early returns)
      if len(data) == 0 {
          return ErrEmptyData
      }
      if !isValid(data) {
          return ErrInvalidData
      }

      // Main logic (happy path)
      result := transform(data)
      return save(result)
  }
  ```

### Code Organization Within Files

- ✅ **Logical grouping with headers:**
  ```go
  // ============================================
  // WebSocket Event Capture
  // ============================================

  func (c *Capture) AddWebSocketEvents() { ... }
  func (c *Capture) GetWebSocketEvents() { ... }

  // ============================================
  // Network Body Capture
  // ============================================

  func (c *Capture) AddNetworkBodies() { ... }
  ```

- ✅ **Related functions together:** Keep getters/setters near their data
- ✅ **Public before private:** Exported functions first, internal helpers after
- ✅ **Constructors at top:** `NewX()` functions at beginning of file

### Blank Lines for Readability

```go
// ✅ Good - Logical sections separated
func process() error {
    // Section 1: Validation
    if err := validate(); err != nil {
        return err
    }

    // Section 2: Processing
    result := transform()

    // Section 3: Storage
    return save(result)
}

// ❌ Bad - No separation
func process() error {
    if err := validate(); err != nil {
        return err
    }
    result := transform()
    return save(result)
}
```

### Indentation & Nesting

- ✅ **Max nesting: 4 levels:** If deeper, extract function
- ✅ **Prefer early returns:** Reduce nesting
  ```go
  // ❌ Bad - Deep nesting
  func process() error {
      if valid {
          if authorized {
              if hasData {
                  if canProcess {
                      // Deep logic
                  }
              }
          }
      }
  }

  // ✅ Good - Early returns
  func process() error {
      if !valid {
          return ErrInvalid
      }
      if !authorized {
          return ErrUnauthorized
      }
      if !hasData {
          return ErrNoData
      }
      if !canProcess {
          return ErrCannotProcess
      }

      // Main logic at top level
      return doProcess()
  }
  ```

### Comments for Clarity

- ✅ **Comment complex algorithms:** If not obvious, explain
- ✅ **Comment non-obvious decisions:** "Why" this approach
- ✅ **Comment gotchas:** Things that might surprise
  ```go
  // Parse cursor format: "timestamp:sequence"
  // Note: timestamp can be RFC3339 or RFC3339Nano, sequence is optional
  // Examples: "2026-01-30T10:15:23Z:42" or "2026-01-30T10:15:23.456Z"
  func parseCursor(cursor string) (Cursor, error) {
      // ...
  }
  ```

- ✅ **Don't comment obvious code:**
  ```go
  // ❌ Bad - States the obvious
  // Set the name
  user.Name = name

  // ✅ Good - No comment needed (code is self-explanatory)
  user.Name = name
  ```

### Magic Numbers & Constants

- ✅ **All magic numbers as named constants:**
  ```go
  // ❌ Bad
  if len(data) > 10240 {
      truncate()
  }

  // ✅ Good
  const MaxDataSize = 10 * 1024 // 10KB

  if len(data) > MaxDataSize {
      truncate()
  }
  ```

- ✅ **Group related constants:** Use const blocks
- ✅ **Document why:** Explain why this value
  ```go
  const (
      // MaxPendingQueries limits queue size to prevent memory growth
      // Value: 5 based on typical extension polling rate (1/sec) and command timeout (30s)
      MaxPendingQueries = 5

      // AsyncCommandTimeout is how long to wait for extension to execute command
      // Value: 30s allows time for page load + script execution
      AsyncCommandTimeout = 30 * time.Second
  )
  ```

### Error Messages (User-Facing)

- ✅ **Clear and specific:**
  ```go
  // ❌ Bad
  return errors.New("invalid input")

  // ✅ Good
  return fmt.Errorf("invalid limit parameter: %d (expected: 1-%d)", limit, maxLimit)
  ```

- ✅ **Include context:** What failed, what values caused it
- ✅ **Suggest fix:** Tell user how to correct it
- ✅ **Consistent error format:** Use same pattern across codebase

### Code Flow Readability

- ✅ **Happy path on the left:** Errors handled with early returns
- ✅ **No pyramids of doom:** Avoid deep if-else nesting
- ✅ **Prefer switch over if-else chains:**
  ```go
  // ✅ Good - Switch is clearer for many conditions
  switch req.Method {
  case "GET":
      return handleGet(req)
  case "POST":
      return handlePost(req)
  case "PUT":
      return handlePut(req)
  default:
      return ErrMethodNotAllowed
  }
  ```

### File Organization for Readability

- ✅ **File header explains purpose:**
  ```go
  // tools_observe.go — Observe tool implementation for MCP.
  // Handles all "observe" tool modes (logs, errors, network, websocket, etc.)
  // Thread-safe: All methods acquire handler locks as needed.
  package main
  ```

- ✅ **Imports organized:** stdlib, external, internal (separated by blank lines)
- ✅ **Constants before vars:** Configuration at top of file
- ✅ **Types before functions:** Data structures defined before operations

---

## 🛠️ Build & Deploy

### Compilation Checks

- ✅ **Compile before committing:** `go build ./...`
- ✅ **Run linters:** `npm run lint`, `go vet ./...`
- ✅ **Type checking:** `npm run typecheck`
- ✅ **All checks automated:** `make quality-gate`

### Dependency Management

- ✅ **Zero production dependencies:** Gasoline rule
- ✅ **Dev dependencies locked:** package-lock.json, go.sum
- ✅ **Security scanning:** govulncheck, npm audit
- ✅ **Document why:** If dependency added, explain in PR

### Configuration Management

- ✅ **Environment variables for config:**
  ```go
  port := getEnvOrDefault("GASOLINE_PORT", "7890")
  ```

- ✅ **Config validation on startup:** Fail fast
- ✅ **Document all config:** What vars exist, what they do
- ✅ **Sensible defaults:** Should work without configuration

---

## 📖 Documentation Standards

### Code Comments

- ✅ **Explain WHY, not WHAT:** Code shows what, comments explain why
- ✅ **Document complex logic:** If it's not obvious, explain
- ✅ **Document tradeoffs:** Why this approach vs alternatives
- ✅ **Update stale comments:** Keep comments in sync with code

### Package Documentation

- ✅ **Every package has doc.go:**
  ```go
  // Package capture provides real-time browser telemetry capture.
  //
  // Core functionality:
  //   - WebSocket event capture
  //   - Network request/response body capture
  //   - User action capture
  //
  // Thread safety: All methods are thread-safe using a single mutex.
  package capture
  ```

### API Documentation

- ✅ **Document all public APIs:** Functions, types, constants
- ✅ **Include examples:** Show how to use the API
- ✅ **Document errors:** What errors can occur and when

---

## 🎯 Architecture & Design Patterns

### SOLID Principles

- ✅ **Single Responsibility:** Each component does one thing
- ✅ **Open/Closed:** Open for extension, closed for modification
- ✅ **Liskov Substitution:** Subtypes are substitutable
- ✅ **Interface Segregation:** Small, focused interfaces
- ✅ **Dependency Inversion:** Depend on abstractions, not concretions

### Design Patterns (Use Appropriately)

- ✅ **Factory:** For complex object creation
- ✅ **Builder:** For objects with many optional fields
- ✅ **Strategy:** For swappable algorithms
- ✅ **Observer:** For event notification
- ✅ **Singleton:** ONLY when truly needed, prefer dependency injection

### Anti-Patterns (Avoid)

- ❌ **God objects:** Components with too many responsibilities
- ❌ **Tight coupling:** Components that can't be changed independently
- ❌ **Premature optimization:** Optimize when measurements show need
- ❌ **Copy-paste:** Extract shared code into functions
- ❌ **Magic numbers:** Use named constants

---

**Last updated:** 2026-02-03
**See also:** [README.md](README.md) — Navigation and index

