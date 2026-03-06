---
title: "Why Gasoline Is Written in Go"
date: 2026-02-07
authors:
  - brenn
tags:
  - architecture
  - engineering
---

Go was chosen for Gasoline's MCP server because it compiles to a single binary, has zero runtime dependencies, and doesn't rot. Here's why that matters for a developer tool you depend on daily.

<!-- more -->

## The Code Rot Problem

You clone a Node.js project from last year. You run `npm install`. It fails. A dependency broke. You update it. Now another dependency is incompatible. You spend an hour fixing a project that worked perfectly 12 months ago.

This is code rot — software that degrades not because its logic changed, but because the ecosystem around it shifted.

In the JavaScript ecosystem, code rot is constant. The average npm package has 79 transitive dependencies. Each one is a ticking clock — an author who might yank the package, a breaking change in a minor version, a deprecated API, a CVE that forces an urgent update.

Gasoline doesn't have this problem. It has zero production dependencies.

## What Zero Dependencies Means

Gasoline's Go server imports nothing outside the Go standard library. No HTTP frameworks. No logging libraries. No JSON-RPC packages. No ORMs.

```go
import (
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "sync"
)
```

That's it. Everything Gasoline does — HTTP serving, JSON-RPC 2.0, ring buffers, cursor pagination, file persistence, rate limiting, multi-client bridging — is built on Go's standard library.

**What this means in practice:**

- **No `go.sum` file to audit.** There are no third-party dependencies to check for vulnerabilities.
- **No supply chain attacks.** You can't compromise what doesn't exist.
- **No version conflicts.** No "package X requires Y v2 but Z requires Y v1."
- **No breaking updates.** Go's compatibility guarantee means code written for Go 1.21 compiles on Go 1.25.

## Single Binary Distribution

`go build` produces a single executable. No runtime required. No interpreter. No virtual machine.

When you run `npx gasoline-mcp`, it downloads a pre-compiled binary for your platform and runs it. There's no:

- `npm install` (no node_modules)
- `pip install` (no Python environment)
- `go install` (no Go toolchain needed)
- `.jar` files (no JVM)

One binary. It runs. That's the deployment story.

Compare this to tools built on Node.js:

| | Node.js MCP Server | Gasoline (Go) |
|---|---|---|
| Runtime required | Node.js 18+ | None |
| Package manager | npm/yarn/pnpm | None |
| Dependencies | Dozens to hundreds | Zero |
| Install time | 10-60 seconds | < 2 seconds (binary download) |
| Disk footprint | 50-200 MB (node_modules) | ~15 MB (single binary) |
| Cold start | 500ms-2s (require/import resolution) | 300-400ms |
| CVE exposure | Every dependency | Go stdlib only |

## Go's Compatibility Guarantee

Go has a forward compatibility guarantee: code written for Go 1.x compiles on all future 1.x versions. This guarantee has held since Go 1.0 in 2012.

In practice, this means:

- Code written today compiles in 5 years without changes
- No "upgrade to the latest framework version" treadmill
- No deprecation warnings that become errors in the next release
- No migration guides to follow every 18 months

For a developer tool that people depend on daily, this stability is a feature, not a constraint.

## Performance That Comes Free

Go compiles to native machine code. There's no interpreter overhead, no JIT warmup, no garbage collection pauses that matter at Gasoline's scale.

Gasoline's performance targets:

| Metric | Target | Why Go helps |
|---|---|---|
| Console intercept overhead | < 0.1ms | No runtime startup per request |
| HTTP endpoint latency | < 0.5ms | `net/http` is battle-tested and fast |
| Cold start | < 600ms | Static binary, no dependency resolution |
| Concurrent clients | 500+ | Goroutines are cheap (~2KB stack each) |
| Memory under load | Predictable | Ring buffers with fixed capacity |

Go's concurrency model (goroutines + channels) makes the multi-client bridge pattern trivial. Each MCP client gets its own goroutine. The ring buffers are protected by `sync.RWMutex`. No thread pool configuration, no async/await complexity, no callback hell.

## Why Not Node.js?

Node.js is great for web applications. But for a local system tool like an MCP server, it has drawbacks:

**Dependency sprawl.** Even a simple HTTP server pulls in Express or Fastify, which pull in dozens of sub-dependencies. Each one needs maintenance.

**Runtime requirement.** Users need Node.js installed. Version mismatches cause issues. Some users have Node 16, some have 22, and the behaviors differ.

**Startup overhead.** Node.js resolves and loads modules at startup. For a CLI tool that needs to be ready in milliseconds, this adds latency.

**Single-threaded concurrency.** Node.js uses an event loop. Under load (500 concurrent MCP clients), this becomes a bottleneck. Go's goroutines scale to thousands of concurrent operations without additional complexity.

## Why Not Rust?

Rust would also work well for Gasoline — single binary, no runtime, excellent performance. The tradeoff is development speed.

Go's simpler type system and garbage collector mean faster iteration. For a product that ships features weekly and maintains a 140-test suite, developer velocity matters more than squeezing the last nanosecond out of a buffer read.

Go is fast enough. And "fast enough with faster development" beats "fastest possible with slower development" for a tool that's still rapidly evolving.

## The Bottom Line

Go gives Gasoline:

- **Zero dependencies** — nothing to break, nothing to patch, nothing to audit
- **Single binary** — download and run, no runtime required
- **Stability** — compiles the same way in 5 years
- **Performance** — native code, goroutine concurrency, predictable memory
- **Fast development** — simple language, fast compilation, easy concurrency

The language choice is a product decision, not just a technical one. When your tool has zero dependencies, it doesn't rot. When it's a single binary, it installs in seconds. When it's stable, users trust it.

That trust compounds over time. And time is the one dependency you can't pin to a version.
