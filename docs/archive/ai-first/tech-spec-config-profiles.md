# Technical Spec: Configuration Profiles

## Purpose

Managing multiple Gasoline configuration options (rate limits, body capture, redaction levels, TTLs) is tedious and error-prone when settings must be adjusted for different contexts — debugging vs. demo vs. production, open development vs. sensitive financial data. Configuration profiles bundle related settings into named presets that can be activated with a single command.

This feature enables "bank mode" — a one-click enterprise setup where an organization can define a restrictive profile once and distribute it, ensuring consistent security posture across all developer workstations.

---

## How It Works

### Profile System

A configuration profile is a named bundle of settings that, when activated, overrides the server's default configuration. Profiles follow an inheritance model: a profile can extend another profile and override specific settings, enabling layered configuration.

The server maintains:
1. **Built-in profiles** — Four standard profiles ship with Gasoline: `default`, `short-lived`, `restricted`, and `paranoid`
2. **Custom profiles** — User-defined profiles stored in `.gasoline/profiles/` that can extend built-in or other custom profiles
3. **Active profile** — The currently active profile (defaults to `default`)
4. **Runtime overrides** — Settings applied on top of the active profile for the current session

When the server starts, it loads the active profile from persistent storage (if available) or falls back to `default`. Profile changes take effect immediately — no server restart required.

### Profile Resolution

When a setting is needed, the server resolves it through this chain:

```
Runtime Override → Active Profile → Parent Profile(s) → Built-in Default
```

Example: If `paranoid` profile is active and extends `restricted`, and a runtime override sets `buffer_ttl_seconds=60`:
1. Check runtime overrides → `buffer_ttl_seconds=60` found, use it
2. For other settings, check `paranoid` profile
3. If not set in `paranoid`, check `restricted` (parent)
4. If not set in `restricted`, use built-in default

---

## Built-in Profiles

### `default`

The standard development profile. Balanced between visibility and performance.

```json
{
  "name": "default",
  "description": "Standard development settings with full capture",
  "settings": {
    "body_capture": {
      "enabled": true,
      "max_size_bytes": 102400,
      "content_types": ["application/json", "text/plain", "text/html", "application/xml"]
    },
    "buffer_limits": {
      "console_entries": 1000,
      "network_entries": 500,
      "network_bodies": 100,
      "websocket_events": 500,
      "actions": 200
    },
    "buffer_ttl_seconds": 3600,
    "rate_limit": {
      "events_per_second": 1000,
      "circuit_breaker_threshold_seconds": 5
    },
    "redaction": {
      "level": "standard",
      "strip_auth_headers": true,
      "strip_cookie_values": true,
      "custom_patterns": []
    },
    "tools": {
      "enabled": "*",
      "disabled": []
    },
    "streaming": {
      "enabled": false,
      "throttle_seconds": 5
    }
  }
}
```

### `short-lived`

For quick debugging sessions where data retention isn't needed. Aggressive TTLs and minimal buffers reduce memory footprint.

```json
{
  "name": "short-lived",
  "extends": "default",
  "description": "Minimal retention for quick debug sessions",
  "settings": {
    "buffer_limits": {
      "console_entries": 200,
      "network_entries": 100,
      "network_bodies": 20,
      "websocket_events": 100,
      "actions": 50
    },
    "buffer_ttl_seconds": 300,
    "body_capture": {
      "max_size_bytes": 10240
    }
  }
}
```

### `restricted`

For environments with sensitive data. Limits tool access and increases redaction.

```json
{
  "name": "restricted",
  "extends": "default",
  "description": "Limited tools and enhanced redaction for sensitive environments",
  "settings": {
    "redaction": {
      "level": "aggressive",
      "strip_auth_headers": true,
      "strip_cookie_values": true,
      "redact_request_bodies": true,
      "redact_response_bodies": true,
      "custom_patterns": [
        {"name": "ssn", "pattern": "\\d{3}-\\d{2}-\\d{4}"},
        {"name": "credit_card", "pattern": "\\b\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}\\b"},
        {"name": "email", "pattern": "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"}
      ]
    },
    "tools": {
      "enabled": ["observe", "analyze"],
      "disabled": ["generate", "browser_action", "execute_javascript", "manage_state"]
    },
    "body_capture": {
      "enabled": true,
      "max_size_bytes": 51200
    },
    "streaming": {
      "enabled": false
    }
  }
}
```

### `paranoid`

Maximum privacy. No body capture, minimal tool access, shortest retention. Use for financial apps, healthcare, or any environment where data leakage is a critical concern.

```json
{
  "name": "paranoid",
  "extends": "restricted",
  "description": "Maximum privacy - no body capture, minimal retention",
  "settings": {
    "body_capture": {
      "enabled": false
    },
    "buffer_limits": {
      "console_entries": 100,
      "network_entries": 50,
      "network_bodies": 0,
      "websocket_events": 50,
      "actions": 25
    },
    "buffer_ttl_seconds": 120,
    "redaction": {
      "level": "maximum",
      "redact_urls": true,
      "redact_query_params": true
    },
    "tools": {
      "enabled": ["observe"],
      "disabled": ["analyze", "generate", "browser_action", "execute_javascript", "manage_state", "query_dom", "highlight_element"]
    }
  }
}
```

---

## Data Model

### Profile Schema

```go
// Profile represents a named configuration bundle
type Profile struct {
    Name        string          `json:"name"`
    Extends     string          `json:"extends,omitempty"`
    Description string          `json:"description"`
    Settings    ProfileSettings `json:"settings"`
    Builtin     bool            `json:"builtin,omitempty"`
    CreatedAt   time.Time       `json:"created_at,omitempty"`
    UpdatedAt   time.Time       `json:"updated_at,omitempty"`
}

// ProfileSettings contains all configurable options
type ProfileSettings struct {
    BodyCapture  *BodyCaptureSettings  `json:"body_capture,omitempty"`
    BufferLimits *BufferLimitSettings  `json:"buffer_limits,omitempty"`
    BufferTTL    *int                  `json:"buffer_ttl_seconds,omitempty"`
    RateLimit    *RateLimitSettings    `json:"rate_limit,omitempty"`
    Redaction    *RedactionSettings    `json:"redaction,omitempty"`
    Tools        *ToolSettings         `json:"tools,omitempty"`
    Streaming    *StreamingSettings    `json:"streaming,omitempty"`
}

// BodyCaptureSettings controls network body capture
type BodyCaptureSettings struct {
    Enabled      *bool    `json:"enabled,omitempty"`
    MaxSizeBytes *int     `json:"max_size_bytes,omitempty"`
    ContentTypes []string `json:"content_types,omitempty"`
}

// BufferLimitSettings controls buffer sizes
type BufferLimitSettings struct {
    ConsoleEntries  *int `json:"console_entries,omitempty"`
    NetworkEntries  *int `json:"network_entries,omitempty"`
    NetworkBodies   *int `json:"network_bodies,omitempty"`
    WebSocketEvents *int `json:"websocket_events,omitempty"`
    Actions         *int `json:"actions,omitempty"`
}

// RateLimitSettings controls ingest rate limiting
type RateLimitSettings struct {
    EventsPerSecond              *int `json:"events_per_second,omitempty"`
    CircuitBreakerThresholdSecs  *int `json:"circuit_breaker_threshold_seconds,omitempty"`
}

// RedactionSettings controls data sanitization
type RedactionSettings struct {
    Level             *string           `json:"level,omitempty"` // "standard", "aggressive", "maximum"
    StripAuthHeaders  *bool             `json:"strip_auth_headers,omitempty"`
    StripCookieValues *bool             `json:"strip_cookie_values,omitempty"`
    RedactRequestBodies  *bool          `json:"redact_request_bodies,omitempty"`
    RedactResponseBodies *bool          `json:"redact_response_bodies,omitempty"`
    RedactURLs        *bool             `json:"redact_urls,omitempty"`
    RedactQueryParams *bool             `json:"redact_query_params,omitempty"`
    CustomPatterns    []RedactionPattern `json:"custom_patterns,omitempty"`
}

// RedactionPattern defines a custom redaction rule
type RedactionPattern struct {
    Name        string `json:"name"`
    Pattern     string `json:"pattern"`
    Replacement string `json:"replacement,omitempty"` // defaults to "[REDACTED]"
}

// ToolSettings controls MCP tool availability
type ToolSettings struct {
    Enabled  interface{} `json:"enabled,omitempty"`  // "*" for all, or []string of tool names
    Disabled []string    `json:"disabled,omitempty"` // tools to disable (takes precedence)
}

// StreamingSettings controls context streaming
type StreamingSettings struct {
    Enabled         *bool    `json:"enabled,omitempty"`
    ThrottleSeconds *int     `json:"throttle_seconds,omitempty"`
    Events          []string `json:"events,omitempty"`
}
```

### Profile Manager State

```go
// ProfileManager handles profile storage, resolution, and activation
type ProfileManager struct {
    builtinProfiles map[string]*Profile  // Immutable built-in profiles
    customProfiles  map[string]*Profile  // User-defined profiles
    activeProfile   string               // Name of currently active profile
    runtimeOverrides *ProfileSettings    // Per-session overrides
    resolvedCache   *ProfileSettings     // Cached fully-resolved settings
    mu              sync.RWMutex
}
```

---

## API Surface

### MCP Tool: `configure` (extended)

The existing `configure` tool gains a new action for profile management.

**Parameters for `action: "profile"`**:
- `profile_action` (required): One of `"activate"`, `"list"`, `"get"`, `"create"`, `"delete"`, `"export"`
- `name`: Profile name (required for activate, get, create, delete, export)
- `profile`: Profile object (required for create)
- `override`: Settings to apply on top of active profile (optional for activate)

**Examples**:

Activate a built-in profile:
```json
{
  "action": "profile",
  "profile_action": "activate",
  "name": "paranoid"
}
```

Activate with runtime overrides:
```json
{
  "action": "profile",
  "profile_action": "activate",
  "name": "restricted",
  "override": {
    "buffer_ttl_seconds": 1800,
    "tools": {
      "enabled": ["observe", "analyze", "generate"]
    }
  }
}
```

List all profiles:
```json
{
  "action": "profile",
  "profile_action": "list"
}
```

Response:
```json
{
  "profiles": [
    {"name": "default", "builtin": true, "description": "Standard development settings with full capture"},
    {"name": "short-lived", "builtin": true, "extends": "default", "description": "Minimal retention for quick debug sessions"},
    {"name": "restricted", "builtin": true, "extends": "default", "description": "Limited tools and enhanced redaction for sensitive environments"},
    {"name": "paranoid", "builtin": true, "extends": "restricted", "description": "Maximum privacy - no body capture, minimal retention"},
    {"name": "acme-bank", "builtin": false, "extends": "paranoid", "description": "ACME Bank compliance profile"}
  ],
  "active": "default"
}
```

Get profile details:
```json
{
  "action": "profile",
  "profile_action": "get",
  "name": "paranoid"
}
```

Create custom profile:
```json
{
  "action": "profile",
  "profile_action": "create",
  "name": "my-team",
  "profile": {
    "extends": "default",
    "description": "Team-specific settings",
    "settings": {
      "buffer_ttl_seconds": 7200,
      "redaction": {
        "custom_patterns": [
          {"name": "internal_id", "pattern": "ACME-\\d{8}"}
        ]
      }
    }
  }
}
```

Export profile (for sharing):
```json
{
  "action": "profile",
  "profile_action": "export",
  "name": "my-team"
}
```

Response includes fully resolved settings:
```json
{
  "profile": {
    "name": "my-team",
    "extends": "default",
    "description": "Team-specific settings",
    "settings": { /* fully merged settings */ }
  },
  "resolved": { /* all settings after inheritance resolution */ }
}
```

### CLI Flags

```bash
# Start server with specific profile
gasoline --profile=paranoid

# Start with profile and override
gasoline --profile=restricted --buffer-ttl=300

# Import a profile file on startup
gasoline --import-profile=/path/to/acme-bank.json

# Export current profile to file
gasoline --export-profile=/path/to/output.json
```

### Environment Variables

```bash
# Set default profile
GASOLINE_PROFILE=paranoid

# Override individual settings (takes precedence over profile)
GASOLINE_BODY_CAPTURE=false
GASOLINE_BUFFER_TTL=300
GASOLINE_REDACTION_LEVEL=maximum
```

---

## Profile Inheritance and Overrides

### Inheritance Chain

Profiles can extend other profiles to create layered configurations. The inheritance chain is resolved at activation time, not at definition time.

```
Custom Profile → Parent Profile → ... → Built-in Profile → Hardcoded Defaults
```

**Circular inheritance detection**: The profile manager rejects profiles that would create circular inheritance. Maximum inheritance depth is 5.

### Merge Semantics

Settings merge using these rules:

1. **Scalar values** (strings, numbers, booleans): Child overrides parent completely
2. **Arrays**: Child replaces parent completely (no merging)
3. **Objects**: Deep merge — child keys override parent keys, parent keys not in child are preserved
4. **Null/omitted**: Inherits from parent

Example:
```json
// Parent (restricted)
{
  "redaction": {
    "level": "aggressive",
    "strip_auth_headers": true,
    "custom_patterns": [{"name": "ssn", "pattern": "..."}]
  }
}

// Child (acme-bank)
{
  "extends": "restricted",
  "redaction": {
    "level": "maximum",
    "custom_patterns": [{"name": "account", "pattern": "..."}]
  }
}

// Resolved
{
  "redaction": {
    "level": "maximum",                    // overridden
    "strip_auth_headers": true,            // inherited
    "custom_patterns": [{"name": "account", "pattern": "..."}]  // replaced (not merged)
  }
}
```

### Runtime Overrides

Runtime overrides are temporary settings applied on top of the active profile for the current server session. They do not modify the profile definition and are lost on server restart.

Use cases:
- Temporarily enabling a tool that's disabled by the profile
- Adjusting TTL for a specific debugging session
- Testing configuration changes before committing to a profile

```json
{
  "action": "profile",
  "profile_action": "activate",
  "name": "paranoid",
  "override": {
    "tools": {
      "enabled": ["observe", "query_dom"]
    }
  }
}
```

---

## Bank Mode: Enterprise Deployment

"Bank mode" is the pattern of distributing a restrictive profile across an organization.

### Profile Distribution

1. **Create the profile** on a reference machine:
```json
{
  "action": "profile",
  "profile_action": "create",
  "name": "acme-bank",
  "profile": {
    "extends": "paranoid",
    "description": "ACME Bank compliance profile - DO NOT MODIFY",
    "settings": {
      "redaction": {
        "level": "maximum",
        "custom_patterns": [
          {"name": "account_number", "pattern": "\\b\\d{10,12}\\b"},
          {"name": "routing_number", "pattern": "\\b\\d{9}\\b"}
        ]
      },
      "tools": {
        "enabled": ["observe"],
        "disabled": ["*"]
      }
    }
  }
}
```

2. **Export the profile**:
```bash
gasoline --export-profile=acme-bank.json
```

3. **Distribute via package manager or config management**:
```bash
# In package.json scripts
"postinstall": "gasoline --import-profile=./config/acme-bank.json"

# Or via environment
GASOLINE_PROFILE_URL=https://internal.acme.com/gasoline/acme-bank.json
```

4. **Enforce via environment**:
```bash
# In CI/CD or developer machine setup
export GASOLINE_PROFILE=acme-bank
export GASOLINE_PROFILE_LOCKED=true  # Prevents profile changes
```

### Profile Locking

When `GASOLINE_PROFILE_LOCKED=true`:
- `configure(action:"profile", profile_action:"activate")` with a different profile returns an error
- `configure(action:"profile", profile_action:"create")` returns an error
- `configure(action:"profile", profile_action:"delete")` returns an error
- Runtime overrides are allowed but logged to audit trail

This enables compliance teams to enforce a baseline while still allowing developers to adjust non-sensitive settings.

---

## Persistence

### Storage Location

Custom profiles are stored in `.gasoline/profiles/<name>.json`. The active profile name is stored in `.gasoline/meta.json` under `active_profile`.

```
.gasoline/
├── meta.json              # includes "active_profile": "acme-bank"
├── profiles/
│   ├── acme-bank.json
│   └── my-team.json
└── ...
```

### Load Order

On server startup:
1. Load built-in profiles (hardcoded)
2. Scan `.gasoline/profiles/` and load custom profiles
3. Read `active_profile` from meta.json
4. If active profile doesn't exist, fall back to `default`
5. Resolve and cache the active profile settings

---

## Security Invariants

1. **Built-in profiles are immutable** — Cannot be modified or deleted via API
2. **Profile names are validated** — Alphanumeric, hyphens, underscores only, max 50 chars
3. **Circular inheritance rejected** — Detected at create/import time
4. **Locked profiles enforced** — When `GASOLINE_PROFILE_LOCKED=true`, all profile mutation operations fail
5. **Audit trail** — All profile changes are logged (who, when, what changed)
6. **Custom patterns validated** — Invalid regex patterns in redaction rules cause profile creation to fail

---

## Edge Cases

- **Missing parent profile**: If a profile extends a non-existent profile, activation fails with a clear error
- **Profile with same name as built-in**: Rejected at creation time
- **Empty settings object**: Valid — inherits everything from parent
- **Server restart during profile change**: The previous active profile is restored from meta.json
- **Concurrent profile activation**: Mutex-protected; last activation wins
- **Profile import with conflicts**: Import fails if a profile with the same name exists (use `--force` to overwrite)
- **Invalid JSON in profile file**: Logged as error, profile skipped, server continues with remaining profiles

---

## Performance Constraints

- Profile activation (including resolution): under 10ms
- Settings lookup (from cache): under 0.01ms
- Profile export (with resolution): under 50ms
- Profile import: under 100ms
- Memory per profile: under 10KB
- Maximum custom profiles: 20

---

## Test Scenarios

### Profile Management

1. List profiles returns all built-in profiles
2. Activate built-in profile → settings change immediately
3. Activate non-existent profile → error
4. Create custom profile → appears in list
5. Create profile extending built-in → inheritance works
6. Create profile with circular inheritance → rejected
7. Create profile with invalid name → rejected
8. Delete custom profile → removed from list
9. Delete built-in profile → rejected
10. Export profile → includes resolved settings

### Inheritance

11. Child overrides parent scalar → child value used
12. Child omits parent field → parent value inherited
13. Child overrides parent array → child array used (not merged)
14. Child overrides parent object → deep merged
15. Three-level inheritance → all levels resolved correctly
16. Parent profile deleted while child exists → child activation fails

### Runtime Overrides

17. Activate with override → override applied on top
18. Override does not modify stored profile
19. Server restart → override lost, base profile restored
20. Override enables disabled tool → tool becomes available
21. Override with invalid setting → rejected, profile unchanged

### Persistence

22. Create profile → file created in .gasoline/profiles/
23. Server restart → custom profiles loaded
24. Server restart → active profile restored
25. Corrupted profile file → skipped with error log
26. Missing .gasoline directory → created on first profile save

### Bank Mode

27. GASOLINE_PROFILE_LOCKED=true → profile activation blocked
28. GASOLINE_PROFILE_LOCKED=true → runtime overrides allowed
29. GASOLINE_PROFILE_LOCKED=true → profile changes logged to audit
30. Import profile via URL → downloaded and installed
31. Import profile with --force → overwrites existing

### Tool Availability

32. Profile disables tool → tool not in MCP tools list
33. Profile enables subset of tools → only those tools available
34. Profile with tools.enabled="*" and tools.disabled=["X"] → all except X available
35. Tool disabled by profile but enabled by override → tool available

---

## File Location

Implementation goes in `cmd/dev-console/profiles.go` with tests in `cmd/dev-console/profiles_test.go`.
