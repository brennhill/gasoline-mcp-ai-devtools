# Security Flagging: Design Decisions Summary

## Quick Answers to Your Questions

### Q: How will Unit42 and Spamhaus data be loaded and used?

**A: Static hardcoded lists, NOT API calls.**

```go
// NO external API calls - privacy-first design
var SuspiciousTLDs = map[string]TLDReputation{
    ".xyz": {
        Severity:  "high",
        Reason:    "Commonly used for malware distribution and phishing",
        AbuseRate: 0.45,
        Source:    "Unit42 2025 Report, Spamhaus Q4 2025",
    },
    // ... (12 TLDs total)
}
```

**Why static?**
- ✅ Works offline (no network dependency)
- ✅ Zero privacy risk (no data leaves localhost)
- ✅ Fast (< 0.1ms per check)
- ✅ Reliable (no rate limits, no API downtime)
- ✅ Free (uses public threat intelligence reports)

### Q: Where do we store responses from Unit42 and Spamhaus?

**A: Nowhere - no API responses to store.**

The data is **compiled into the binary** from publicly available threat intelligence reports. It's just a Go map in `cmd/dev-console/security.go`.

Security flags (results of analysis) are stored in-memory:
```go
type Capture struct {
    securityFlags []SecurityFlag // Max 1000 flags (~300KB)
    flagCache     map[string][]SecurityFlag // For performance
}
```

**No persistent storage needed:**
- Flags are ephemeral (tied to current browsing session)
- Regenerated on every page load
- No need to persist across server restarts

### Q: What if those services are down?

**A: Not applicable - no external services used.**

Since we're using static hardcoded lists:
- ✅ Always available (no downtime)
- ✅ No rate limiting
- ✅ No network dependency
- ✅ No authentication/API keys needed

### Q: What else do we need to consider?

See comprehensive edge case analysis in [security-flagging-edge-cases.md](security-flagging-edge-cases.md).

**Top 10 critical edge cases:**

1. **Data URLs / Blob URLs** - Skip or extract origin correctly
2. **WebSocket URLs** - Convert `ws://` to `http://` for CSP
3. **Wildcard CDN patterns** - `*.cloudfront.net` for rotating subdomains
4. **Localhost in production** - Flag as CRITICAL warning
5. **Mixed content** - HTTP resources on HTTPS pages (high severity)
6. **False positives** - Whitelist mechanism for legitimate .xyz domains
7. **Missing timing data** - HAR export handles PerformanceResourceTiming limitations
8. **Incomplete data** - Confidence scoring based on entry count and page age
9. **Performance** - Cache flags per origin (avoid re-checking)
10. **Browser compatibility** - Check `performance.getEntriesByType` exists

---

## Update Strategy

**Frequency:** Quarterly (or as needed for active threats)

**Process:**
1. Review public threat intelligence reports (Unit42, Spamhaus, ICANN)
2. Update `cmd/dev-console/security.go` with new data
3. Bump `ThreatIntelVersion` constant (e.g., "2026.1" → "2026.2")
4. Run tests to verify no false positives
5. Document changes in CHANGELOG
6. Release with version bump (minor for quarterly, patch for emergency)

**Sources (all public, free):**
- Unit42 Threat Landscape Report (annual): https://www.paloaltonetworks.com/unit42/threat-research
- Spamhaus TLD Reputation List (quarterly): https://www.spamhaus.org/statistics/tlds/
- ICANN Registry Abuse Reports (monthly): https://www.icann.org/resources/pages/abuse-2013-05-03-en
- Google Safe Browsing Statistics (weekly): https://transparencyreport.google.com/safe-browsing/overview

---

## CSP Generation: Key Design Points

### 1. Wildcard CDN Patterns

**Problem:** CDNs use rotating subdomains
```
https://d1a2b3c4.cloudfront.net  // Today
https://e5f6g7h8.cloudfront.net  // Tomorrow
```

**Solution:** Normalize to wildcard
```go
CDNWildcardPatterns = map[string]string{
    "cloudfront.net": "https://*.cloudfront.net",
    "s3.amazonaws.com": "https://*.s3.amazonaws.com",
}
```

**Output:** `script-src 'self' https://*.cloudfront.net`

### 2. Localhost Detection

**Problem:** Dev origins in production CSP

**Solution:** Separate CSP modes
```go
type CSPMode string
const (
    CSPModeDevelopment CSPMode = "development" // Allow localhost
    CSPModeProduction  CSPMode = "production"  // Flag localhost as CRITICAL
)
```

**Production output:**
```json
{
  "warnings": [
    "⚠️  CRITICAL: Localhost origin http://localhost:3000 detected. Remove before deployment."
  ]
}
```

### 3. Confidence Scoring

**Problem:** Incomplete data if page hasn't fully loaded

**Solution:** Calculate confidence
```go
func calculateCSPConfidence(entriesCount int, pageAge time.Duration) string {
    if entriesCount < 10 || pageAge < 5*time.Second {
        return "low"
    }
    if entriesCount < 50 || pageAge < 30*time.Second {
        return "medium"
    }
    return "high"
}
```

**Low confidence output:**
```json
{
  "confidence": "low",
  "warnings": [
    "⚠️  Low confidence - only 5 network requests captured. Navigate the application fully for complete CSP."
  ]
}
```

### 4. Report-Only vs Enforcing

**Problem:** Breaking existing app with strict CSP

**Solution:** Generate both headers
```json
{
  "enforcing_header": "Content-Security-Policy: ...",
  "report_only_header": "Content-Security-Policy-Report-Only: ...",
  "recommendation": "Start with Report-Only mode to test, then switch to Enforcing mode."
}
```

---

## Security Flagging: Multi-Layer Defense

### Detection Algorithms (5 layers)

1. **Suspicious TLD** - 12 known malicious TLDs (static list)
2. **Non-Standard Port** - Flags unusual ports (excluding common dev ports)
3. **Mixed Content** - HTTP resources on HTTPS pages
4. **IP Address Origin** - Direct IP access (bypasses DNS security)
5. **Typosquatting** - Levenshtein distance to known CDNs

### Severity Levels

| Severity | Description | Example |
|----------|-------------|---------|
| **critical** | Free TLDs with >70% abuse rate | `.tk`, `.ml`, `.ga`, `.cf`, `.gq` |
| **high** | TLDs with 40-70% abuse rate | `.xyz`, `.top`, `.site` |
| **medium** | TLDs with 20-40% abuse rate | `.club`, `.work` |
| **low** | TLDs with 10-20% abuse rate (not flagged by default) | `.info`, `.biz` |

### Whitelist Mechanism

**Built-in whitelist:**
```go
var KnownLegitimateOrigins = map[string]string{
    "https://gen.xyz":      "Legitimate URL shortener",
    "https://alphabet.xyz": "Google parent company",
}
```

**User whitelist:** `~/.gasoline/security.json`
```json
{
  "whitelisted_origins": [
    "https://my-startup.xyz",
    "https://my-app.top"
  ],
  "min_flagging_severity": "medium"
}
```

---

## Performance Characteristics

### Security Flagging Performance

**Without caching:** 5 checks × 1000 origins = 5000 operations (~500ms)

**With caching:** O(1) lookup per origin (~5ms for 1000 origins)

```go
// Cache flags per origin (100x speedup)
type SecurityFlagCache struct {
    cache map[string][]SecurityFlag
}
```

### Memory Usage

| Component | Size per Entry | Max Entries | Total Memory |
|-----------|---------------|-------------|--------------|
| Network waterfall | 500 bytes | 1000 (configurable) | 500KB |
| Security flags | 300 bytes | 1000 | 300KB |
| Flag cache | 50 bytes | 1000 | 50KB |
| **Total** | | | **~850KB** |

**Verdict:** Negligible memory impact

### Network Overhead

- **Frequency:** Extension POSTs every 10 seconds
- **Payload:** ~1KB per 10 resources = 6KB for typical page
- **Bandwidth:** < 1KB/s average (negligible)

---

## Comparison: Static vs Dynamic Threat Intelligence

| Factor | Static Lists (CHOSEN) | Dynamic APIs (REJECTED) |
|--------|----------------------|------------------------|
| **Privacy** | ✅ Zero data leakage | ❌ Sends browsing origins to third parties |
| **Offline** | ✅ Works offline | ❌ Requires network connection |
| **Performance** | ✅ < 0.1ms per check | ❌ 50-200ms per API call |
| **Cost** | ✅ Free | ❌ $500-5000/month for commercial APIs |
| **Rate limits** | ✅ None | ❌ 100-1000 requests/day typical |
| **Reliability** | ✅ Always available | ❌ API downtime, rate limit errors |
| **Complexity** | ✅ Simple map lookups | ❌ API keys, retry logic, error handling |
| **Data freshness** | ⚠️ Quarterly updates | ✅ Real-time |
| **Scope** | ⚠️ TLD-level only | ✅ Domain-specific reputation |

**Winner:** Static lists. Privacy and offline operation are non-negotiable for Gasoline.

---

## Implementation Checklist

Before starting Phase 1 implementation, ensure:

- [ ] Read [security-flagging-edge-cases.md](security-flagging-edge-cases.md) for full edge case analysis
- [ ] Review implementation edge cases checklist in [network-capture-complete.md](network-capture-complete.md)
- [ ] Understand all 10 critical edge cases (Data URLs, WebSockets, wildcards, localhost, mixed content, false positives, HAR limitations, incomplete data, performance, browser compat)
- [ ] Plan for quarterly threat intel updates
- [ ] Set up unit tests for each security check function
- [ ] Set up integration tests with demo site (poisoned dependency)
- [ ] Confirm no external API calls (grep codebase for http.Get, http.Post in security.go)

**Start implementation:** [network-capture-complete.md Phase 1](network-capture-complete.md#phase-1-server-side-foundation-week-1)
