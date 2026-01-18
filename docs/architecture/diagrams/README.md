# Architecture Diagrams

Visual guides to understanding Gasoline's core architecture.

## Quick Navigation

### 🏗️ [System Architecture](system-architecture.md)
**Start here** for overall system understanding.
- C4 context and container diagrams
- Component responsibilities
- Data flows (event capture + browser control)
- Deployment model
- Technology stack

### 🔄 [Async Queue-and-Poll Flow](async-queue-flow.md)
**Core pattern** that enables non-blocking browser automation.
- End-to-end sequence diagram
- Timeout handling
- Queue state machine
- Multi-client isolation
- Performance characteristics

### 🎯 [Correlation ID Lifecycle](correlation-id-lifecycle.md)
**How AI agents track command status** (pending/complete/expired).
- State transitions
- AI polling patterns
- Data structures
- Lock hierarchy
- Troubleshooting guide

### 🛡️ [5-Layer Architectural Protection](5-layer-protection.md)
**Defense-in-depth system** preventing architecture deletion.
- All 5 enforcement layers
- Protection matrix
- Bypass requirements
- Cost/benefit analysis
- Historical context

### 🔥 [Flame Flicker Visual Indicator](flame-flicker-visual.md)
**Browser tab favicon animation** showing AI Pilot active state.
- Visual state machine
- 8-frame animation sequence
- Message flow (popup → background → content)
- SVG structure + performance
- Design rationale

## Diagram Types

### Mermaid Sequence Diagrams
- Async queue-and-poll flow
- Correlation ID polling
- Flame flicker message flow

### Mermaid State Machines
- Queue states (pending/complete/expired)
- Correlation ID lifecycle
- Flame visual states

### Mermaid Flowcharts
- 5-layer protection
- Data capture flow
- Browser control flow

### C4 Architecture Diagrams
- System context
- Container details
- Component relationships

## Usage

All diagrams use [Mermaid](https://mermaid.js.org/) syntax, which **renders automatically on GitHub**.

To view locally:
1. Install Mermaid CLI: `npm install -g @mermaid-js/mermaid-cli`
2. Generate PNG: `mmdc -i async-queue-flow.md -o async-queue-flow.png`

Or use VS Code extension: [Mermaid Preview](https://marketplace.visualstudio.com/items?itemName=bierner.markdown-mermaid)

## Contributing

When adding new diagrams:
- Use consistent color scheme (see existing diagrams)
- Include both overview + detailed views
- Add "References" section linking to code
- Update this README

## Color Scheme

Standard Gasoline colors used in diagrams:

| Color | Hex | Usage |
|-------|-----|-------|
| 🟠 Orange | `#fb923c` | Primary elements, warnings |
| 🟡 Yellow | `#fde047` | Secondary elements, in-progress |
| 🟢 Green | `#3fb950` | Success states, safe operations |
| 🔵 Blue | `#58a6ff` | Info, neutral states |
| 🔴 Red | `#f85149` | Errors, failures, dangerous |
| ⚫ Dark | `#1a1a1a` | Backgrounds, inactive |

## References

- [ADR Index](../README.md)
- [Codebase Canon](../../core/codebase-canon-v5.3.md)
- [Feature Documentation](../../features/README.md)
