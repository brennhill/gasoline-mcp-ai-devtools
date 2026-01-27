# Security Boundary: LLM Trust Model

## ⚠️ CRITICAL: Read This Before Implementation

**This document defines the security boundary between AI agents (LLMs) and persistent security configuration. Failure to follow these rules creates a vulnerability where compromised or poisoned LLMs can manipulate security whitelists.**

---

## Threat Model

### Attack Scenario

```
┌──────────────────────────────┐
│ Compromised/Poisoned LLM     │
│                              │
│ Possible causes:             │
│ - Model poisoning attack     │
│ - Prompt injection           │
│ - Malicious MCP client       │
│ - Supply chain compromise    │
└──────────────┬───────────────┘
               │
               │ MCP Tool Call
               ▼
┌──────────────────────────────┐
│ generate({                   │
│   format: "csp",             │
│   add_to_whitelist: [        │  ❌ MUST BLOCK THIS
│     "https://evil.xyz"       │
│   ]                          │
│ })                           │
└──────────────┬───────────────┘
               │
               ▼
┌──────────────────────────────┐
│ ~/.gasoline/security.json    │
│                              │
│ {                            │
│   "whitelisted_origins": [   │  ❌ Evil origin now hidden
│     "https://evil.xyz"       │     from future scans
│   ]                          │
│ }                            │
└──────────────────────────────┘
```

### What the Attacker Gains

1. **Whitelist injection** - Malicious origin added to permanent whitelist → future CSP scans don't flag it
2. **Severity manipulation** - Lower `min_flagging_severity` to hide medium/high threats
3. **Config corruption** - Delete legitimate whitelist entries or corrupt config
4. **Persistent compromise** - Changes persist across sessions until human reviews config file

---

## Security Principle: Human-in-the-Loop

**Core rule:** All persistent security configuration changes REQUIRE human review.

**Rationale:**
- LLMs are untrusted by design (can be poisoned, prompted, or compromised)
- Security decisions have long-term consequences (persist across sessions)
- Humans are the security boundary (can verify origin legitimacy)
- Manual file editing creates audit trail (git history, file timestamps)

---

## Trust Boundaries

### ✅ TRUSTED

1. **Static threat intelligence** (hardcoded in source code)
   - Curated TLD reputation lists
   - Known CDN patterns
   - Version-controlled and audited

2. **Human decisions** (manual actions)
   - Editing `~/.gasoline/security.json`
   - Interactive CLI confirmations (human present)
   - Command-line flags (human-initiated)

3. **Gasoline server code** (zero-trust architecture)
   - Localhost-only binding
   - Read-only MCP operations for security config
   - Audit logging

### ❌ UNTRUSTED

1. **LLM tool calls** (via MCP protocol)
   - Can be manipulated by prompt injection
   - May originate from compromised model
   - No way to verify intent

2. **MCP clients** (remote or local)
   - Could be malicious
   - Could be compromised via supply chain
   - Cannot distinguish legitimate from malicious requests

3. **Network data** (even from localhost)
   - Could be spoofed
   - Could be MITM'd (though localhost-only reduces this)

---

## Implementation Rules

### Rule 1: No MCP Tool Can Modify Persistent Security Config

**BLOCKED operations (dangerous):**

```javascript
// ❌ NO: Cannot modify ~/.gasoline/security.json via MCP
configure({
  action: "add_to_whitelist",
  origins: ["https://evil.xyz"]
})

// ❌ NO: Cannot change security thresholds via MCP
configure({
  action: "set_min_severity",
  min_severity: "critical"  // Would hide medium/high flags
})

// ❌ NO: Cannot delete whitelist entries via MCP
configure({
  action: "clear_whitelist"
})
```

**ALLOWED operations (safe):**

```javascript
// ✅ YES: Read-only view of current config
observe({
  what: "security_config"
})

// ✅ YES: Session-only override (not persisted)
generate({
  format: "csp",
  whitelist_override: ["https://temp.xyz"]  // Expires after this call
})

// ✅ YES: Flag suppression for analysis (session-only)
generate({
  format: "csp",
  suppress_flags: ["suspicious_tld"]  // One-time suppression
})
```

### Rule 2: Persistent Changes Require Manual Human Action

**ONLY methods to update `~/.gasoline/security.json`:**

1. **Manual file edit** (primary method)
   ```bash
   # Human reviews and manually edits config
   vim ~/.gasoline/security.json
   ```

2. **Interactive CLI prompt** (secondary method, CLI mode only)
   ```bash
   # CLI mode with human present
   $ ./gasoline

   > generate({format: "csp"})

   ⚠️  Suspicious origin: https://my-startup.xyz
       TLD .xyz: 45% abuse rate

   [?] Is this legitimate? [y/N]: y
   [?] Add to persistent whitelist? [Y/n]: y

   ✅ Added to ~/.gasoline/security.json
      Please review this file to confirm.
   ```

3. **Command-line flag** (for CI/testing)
   ```bash
   # Explicit CLI argument (human-initiated)
   ./gasoline --whitelist-origin "https://my-startup.xyz"
   ```

**NEVER via MCP tool call.**

### Rule 3: Session-Only Overrides Are Clearly Labeled

**Pattern for LLM-initiated temporary overrides:**

```go
// CSP generation with temporary whitelist
type CSPParams struct {
    Mode              string   `json:"mode"`
    WhitelistOverride []string `json:"whitelist_override"` // Session-only
    SuppressFlags     []string `json:"suppress_flags"`     // Session-only
}
```

**Output MUST warn user:**

```json
{
  "policy": "script-src 'self' https://my-startup.xyz",
  "warnings": [
    "⚠️  SECURITY: Temporary whitelist override applied (session-only)",
    "   Origin: https://my-startup.xyz",
    "   Source: MCP tool parameter",
    "   Action: Review origin legitimacy before permanent whitelist",
    "",
    "💡 To permanently whitelist (after human review):",
    "   1. Verify origin is legitimate and trusted",
    "   2. Edit ~/.gasoline/security.json manually",
    "   3. Add to 'whitelisted_origins' array"
  ],
  "audit": {
    "session_overrides": ["https://my-startup.xyz"],
    "persistent_whitelist": [],
    "override_source": "mcp_tool_parameter"
  }
}
```

### Rule 4: Audit All Security Decisions

**Log every security-related action:**

```go
// File: cmd/dev-console/audit.go

type SecurityAuditEvent struct {
    Timestamp       time.Time `json:"timestamp"`
    Action          string    `json:"action"`          // "csp_generated", "flag_suppressed", "whitelist_override"
    Origin          string    `json:"origin,omitempty"`
    Reason          string    `json:"reason"`
    Persistent      bool      `json:"persistent"`      // false for session-only
    Source          string    `json:"source"`          // "mcp", "cli", "config_file"
    MCPSessionID    string    `json:"mcp_session_id,omitempty"`
}

// Append to ~/.gasoline/security-audit.jsonl
func LogSecurityEvent(event SecurityAuditEvent) {
    // User can review what LLM attempted
}
```

**Example audit log:**

```jsonl
{"timestamp":"2026-01-27T14:30:00Z","action":"whitelist_override","origin":"https://my-startup.xyz","reason":"CSP generation","persistent":false,"source":"mcp","mcp_session_id":"sess-123"}
{"timestamp":"2026-01-27T14:35:00Z","action":"whitelist_added","origin":"https://my-startup.xyz","reason":"user confirmed legitimate","persistent":true,"source":"cli"}
```

### Rule 5: Detect MCP Mode vs. CLI Mode

**Implementation:**

```go
// File: cmd/dev-console/mode.go

var (
    isMCPMode         bool
    isInteractive     bool
)

func InitMode() {
    // Detect if running as MCP server (stdin/stdout used for JSON-RPC)
    if os.Getenv("MCP_MODE") == "1" || isStdinStdoutPipe() {
        isMCPMode = true
        isInteractive = false
    } else {
        isMCPMode = false
        isInteractive = isatty.IsTerminal(os.Stdin.Fd())
    }
}

func IsMCPMode() bool {
    return isMCPMode
}

func IsInteractiveTerminal() bool {
    return isInteractive
}

// AddToWhitelist blocks if in MCP mode
func AddToWhitelist(origin string) error {
    if IsMCPMode() {
        return errors.New("security config updates require human review - edit ~/.gasoline/security.json manually")
    }

    if !IsInteractiveTerminal() {
        return errors.New("not in interactive mode - edit ~/.gasoline/security.json manually")
    }

    // Prompt user for confirmation
    fmt.Printf("⚠️  Add %s to permanent whitelist? [y/N]: ", origin)
    var response string
    fmt.Scanln(&response)

    if strings.ToLower(response) != "y" {
        return errors.New("cancelled by user")
    }

    // Proceed with update
    // ...
}
```

---

## MCP Tool Design: Security-Safe API

### Safe MCP Tool Schema

```json
{
  "name": "generate",
  "description": "Generate CSP, HAR, or other security artifacts",
  "inputSchema": {
    "type": "object",
    "properties": {
      "format": {
        "type": "string",
        "enum": ["csp", "har", "sarif"]
      },
      "whitelist_override": {
        "type": "array",
        "items": {"type": "string"},
        "description": "SESSION-ONLY temporary whitelist (not persisted to disk). For permanent whitelist, user must edit ~/.gasoline/security.json manually."
      },
      "suppress_flags": {
        "type": "array",
        "items": {"type": "string"},
        "description": "SESSION-ONLY flag suppression (not persisted). For permanent suppression, user must edit config manually."
      }
    }
  }
}
```

**Key points:**
- Parameter names include "override" (temporary nature)
- Description explicitly states "SESSION-ONLY" and "not persisted"
- Instructions tell user how to make permanent changes

### Blocked MCP Operations

**DO NOT expose these via MCP tools:**

```go
// ❌ NEVER implement these in MCP tool handlers
func HandleAddToWhitelist(params map[string]interface{}) error {
    // NO: This allows LLM to modify persistent config
}

func HandleSetMinSeverity(params map[string]interface{}) error {
    // NO: This allows LLM to hide threats
}

func HandleClearWhitelist(params map[string]interface{}) error {
    // NO: This allows LLM to delete legitimate entries
}
```

---

## Config File Format

**Location:** `~/.gasoline/security.json` (or `./.gasoline/security.json` for project-specific)

**Schema:**

```json
{
  "version": "1.0",
  "whitelisted_origins": [
    "https://my-startup.xyz",
    "https://my-app.top"
  ],
  "min_flagging_severity": "medium",
  "notes": {
    "https://my-startup.xyz": "Our company domain (verified 2026-01-27)",
    "https://my-app.top": "Staging environment (verified 2026-01-27)"
  }
}
```

**Load order (cascading):**

1. Built-in hardcoded whitelist (source code)
2. User home config: `~/.gasoline/security.json`
3. Project config: `./.gasoline/security.json` (overrides home config)
4. MCP tool parameters (highest precedence, session-only)

**Atomic writes:**

```go
// Write to temp file first, then atomic rename
func SaveSecurityConfig(config *SecurityConfig) error {
    configPath := filepath.Join(homeDir, ".gasoline", "security.json")
    tempPath := configPath + ".tmp"

    data, _ := json.MarshalIndent(config, "", "  ")

    // Write to temp file
    if err := os.WriteFile(tempPath, data, 0644); err != nil {
        return err
    }

    // Atomic rename (prevents partial writes)
    if err := os.Rename(tempPath, configPath); err != nil {
        return err
    }

    return nil
}
```

---

## Documentation Requirements

### Add to All Security Tool Documentation

```markdown
## ⚠️ SECURITY BOUNDARY WARNING

Gasoline's security features (CSP generation, threat flagging) are designed to
DETECT threats, not to be controlled by potentially compromised LLMs.

**Trust Model:**
- ✅ TRUST: Gasoline's static threat intelligence (hardcoded, version-controlled)
- ✅ TRUST: Human decisions (manual config edits, CLI confirmations)
- ❌ DO NOT TRUST: LLM tool calls to modify security config

**Why?**
A poisoned or compromised LLM could attempt to whitelist malicious origins via MCP
tool calls. To prevent this, all persistent security config changes require human review.

**Safe LLM Usage:**
- LLM CAN read security config and generate CSP
- LLM CAN suggest temporary overrides (session-only, clearly labeled)
- LLM CANNOT modify persistent whitelist (~/.gasoline/security.json)

**To Permanently Whitelist an Origin:**
1. Review security flag and verify origin is legitimate
2. Manually edit `~/.gasoline/security.json`
3. Add origin to `whitelisted_origins` array with note
4. Restart Gasoline server to load new config

**Audit Trail:**
All security decisions are logged to `~/.gasoline/security-audit.jsonl` for review.
```

---

## Implementation Checklist

Before merging any security-related code, verify:

### Code Review Checklist

- [ ] **No MCP tool modifies `~/.gasoline/security.json`**
- [ ] **Session-only overrides clearly labeled in output**
- [ ] **Audit log records all security decisions with source**
- [ ] **MCP mode detection prevents interactive prompts**
- [ ] **Interactive CLI prompts require explicit confirmation**
- [ ] **Config file writes are atomic (no partial writes)**
- [ ] **Parameter names include "override" or "temporary"**
- [ ] **Documentation includes security boundary warning**
- [ ] **Error messages guide user to manual config edit**
- [ ] **Test coverage for blocked operations**

### Security Test Cases

```go
// Test: MCP mode blocks whitelist modifications
func TestMCPModeBlocksWhitelistAdd(t *testing.T) {
    os.Setenv("MCP_MODE", "1")
    err := AddToWhitelist("https://evil.xyz")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "human review required")
}

// Test: Session-only overrides don't persist
func TestSessionOverrideNotPersisted(t *testing.T) {
    // Generate CSP with override
    csp := GenerateCSP(CSPParams{
        WhitelistOverride: []string{"https://temp.xyz"},
    })

    // Verify override applied
    assert.Contains(t, csp.Policy, "temp.xyz")

    // Generate CSP again without override
    csp2 := GenerateCSP(CSPParams{})

    // Verify override NOT persisted
    assert.NotContains(t, csp2.Policy, "temp.xyz")
}

// Test: Config file not modified by MCP calls
func TestMCPCallsDoNotModifyConfigFile(t *testing.T) {
    // Record initial config file hash
    initialHash := md5sum("~/.gasoline/security.json")

    // Make MCP calls
    GenerateCSP(CSPParams{WhitelistOverride: []string{"https://evil.xyz"}})

    // Verify config file unchanged
    finalHash := md5sum("~/.gasoline/security.json")
    assert.Equal(t, initialHash, finalHash)
}
```

---

## Summary

| Action | MCP Tool | Interactive CLI | Manual Edit | Permitted? |
|--------|----------|----------------|-------------|-----------|
| Generate CSP | ✅ Yes | ✅ Yes | N/A | Always allowed |
| Session-only override | ✅ Yes (with warnings) | ✅ Yes | N/A | Allowed |
| View security config | ✅ Yes (read-only) | ✅ Yes | ✅ Yes | Always allowed |
| Add to persistent whitelist | ❌ **BLOCKED** | ✅ With confirmation | ✅ Yes | Human-only |
| Modify min severity | ❌ **BLOCKED** | ✅ With confirmation | ✅ Yes | Human-only |
| Clear whitelist | ❌ **BLOCKED** | ✅ With confirmation | ✅ Yes | Human-only |

**The security boundary is between LLM suggestions (session-only, audited) and persistent security decisions (human-only, file-based).**

This matches the AI Web Pilot pattern:
- LLM can REQUEST actions → User must ENABLE execution
- LLM can SUGGEST overrides → User must CONFIRM persistence

---

## References

- AI Web Pilot Security Model: [architecture.md](../../.claude/docs/architecture.md#ai-web-pilot-security-model)
- Network Capture Spec: [network-capture-complete.md](network-capture-complete.md)
- Edge Cases: [security-flagging-edge-cases.md](security-flagging-edge-cases.md)
