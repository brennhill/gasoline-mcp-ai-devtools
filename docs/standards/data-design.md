# Data Design Standards

> **Designing data models, types, functions, and dependency injection at top 1% quality**

**Scope:** Data structure design, type safety, functions/methods, and state management patterns that form the foundation of every feature.

**Related Standards:**
- [validation-patterns.md](validation-patterns.md) — Input validation at system boundaries
- [code-quality.md](code-quality.md) — Type safety and code organization
- [memory-and-performance.md](memory-and-performance.md) — Memory safety and performance implications
- [api-and-security.md](api-and-security.md) — API design patterns

---

## 🗂️ Data Models & Objects (Structs/Types)

### Naming

- ✅ **Clear semantic names:** `UserSession` not `Sess`, `NetworkRequest` not `NetReq`
- ✅ **Follow conventions:**
  - Go structs: PascalCase (`RequestBody`)
  - TypeScript interfaces: PascalCase (`NetworkEvent`)
  - Fields: camelCase in TS, PascalCase in Go
- ✅ **Avoid abbreviations** unless domain-standard (HTTP, API, ID okay)

### Documentation

- ✅ **Each field has comment** explaining:
  - **What** it represents
  - **Where** the data comes from (source)
  - **When** it's populated (lifecycle)
  - **Units** if applicable (milliseconds, bytes, etc.)

**Example:**
```go
type NetworkBody struct {
    // Timestamp when request was captured (RFC3339 format)
    // Source: Browser's performance.now() converted to ISO string
    Timestamp string `json:"ts"`

    // HTTP method (GET, POST, PUT, DELETE, etc.)
    // Source: Fetch API request.method property
    Method string `json:"method"`

    // Request duration in milliseconds
    // Source: Calculated from performance.timing
    Duration int `json:"duration"`
}
```

### Design Principles

- ✅ **Flexible for extension:** Use optional fields, versioned schemas
- ✅ **Single Responsibility:** Each struct has one clear purpose
- ✅ **Composition over inheritance:** Embed structs rather than complex hierarchies
- ✅ **Immutable where possible:** Mark fields as readonly/const if they shouldn't change

### Constants & Defaults

- ✅ **Magic values as named constants:**
  ```go
  const (
      DefaultBufferSize = 1000
      MaxBodySize = 16 * 1024 // 16KB
      RequestTimeout = 30 * time.Second
  )
  ```
- ✅ **Document why** each constant has its value
- ✅ **Group related constants** in const blocks

---

## 🔧 Functions & Methods

### Naming

- ✅ **Semantically clear names:** Function name should describe WHAT it does
  - Good: `ParseNetworkRequest()`, `ValidateUserInput()`, `CalculateMemoryUsage()`
  - Bad: `Process()`, `Handle()`, `DoStuff()`
- ✅ **Verb-based names:** Functions do things, name them with verbs
- ✅ **Avoid generic names:** `Get`, `Set`, `Process` are too vague without context

### Function Design

- ✅ **Single Responsibility:** One function, one job
- ✅ **Small functions:** Target < 50 lines, ideally < 30
- ✅ **Pure functions where possible:** Same input = same output, no side effects
- ✅ **Limit parameters:** Max 3-4 parameters, use options struct if more
- ✅ **Return errors, don't panic:** Go: return error; TypeScript: throw Error

### Documentation

- ✅ **Function comment explains:**
  - **What** it does (one sentence)
  - **Why** it exists (if not obvious)
  - **Parameters:** What each parameter means
  - **Returns:** What it returns and any error conditions
  - **Side effects:** If it mutates state, say so
  - **Thread safety:** If it's thread-safe or requires locking

**Example:**
```go
// AddNetworkBodies adds captured HTTP request/response bodies to the buffer.
// Enforces memory limits before adding and tags entries with active test IDs.
// Thread-safe: Acquires c.mu lock for the duration of the operation.
//
// Parameters:
//   bodies - Network bodies from extension (source: fetch wrapper)
//
// Side effects:
//   - Modifies c.networkBodies ring buffer
//   - May trigger memory eviction if limits exceeded
//   - Updates c.networkTotalAdded counter
func (c *Capture) AddNetworkBodies(bodies []NetworkBody) {
    // ...
}
```

### Additional Suggestions for Functions

**Complexity:**
- ✅ **Cyclomatic complexity < 10:** If function has many branches, split it
- ✅ **Max nesting depth: 4 levels:** Deeper = harder to understand
- ✅ **Early returns:** Use guard clauses, avoid deep nesting

**Testability:**
- ✅ **Dependencies injected:** Don't hardcode dependencies
- ✅ **Avoid global state:** Pass state as parameters or receivers
- ✅ **Time/random as parameters:** For testing, inject time.Now, random source

**Validation:**
- ✅ **Validate inputs** at function entry
- ✅ **Return errors** for invalid inputs, don't silently proceed
- ✅ **Document preconditions** if any

**Example:**
```go
// calculateMemoryUsage estimates memory usage for a ring buffer.
// Returns 0 if buffer is nil (defensive).
//
// Complexity: O(n) where n is number of entries
// Performance: < 1ms for buffers up to 10,000 entries
func calculateMemoryUsage(entries []Entry) int64 {
    // Guard: Handle nil/empty
    if len(entries) == 0 {
        return 0
    }

    // Calculate
    total := int64(0)
    for i := range entries {
        total += entrySize(&entries[i])
    }

    return total
}
```

---

## 📦 Dependency Injection & State Management

### Dependency Injection

- ✅ **Inject dependencies, don't hardcode:**
  ```go
  // Bad
  func process() {
      db := sql.Open("postgres", hardcodedURL)
  }

  // Good
  func NewProcessor(db Database) *Processor {
      return &Processor{db: db}
  }
  ```
- ✅ **Use interfaces for dependencies:** Makes testing easier
- ✅ **Constructor functions:** `NewX()` creates properly initialized instances

### State Management

- ✅ **State in injectable container:** For testing, pass state as dependency
- ✅ **Minimal global state:** Avoid global variables
- ✅ **Immutable where possible:** Reduce mutation, easier to reason about
- ✅ **Document state ownership:** Who creates, who modifies, who destroys

**Example:**
```go
// ProcessorConfig is injectable for testing
type ProcessorConfig struct {
    MaxRetries int
    Timeout    time.Duration
    Logger     Logger
}

// Processor has all dependencies injected
type Processor struct {
    config ProcessorConfig
    db     Database
    cache  Cache
}

// NewProcessor creates processor with injected dependencies
func NewProcessor(config ProcessorConfig, db Database, cache Cache) *Processor {
    return &Processor{
        config: config,
        db:     db,
        cache:  cache,
    }
}
```

---

**Last updated:** 2026-02-03
**See also:** [README.md](README.md) — Navigation and index

