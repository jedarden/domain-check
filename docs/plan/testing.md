# Testing & Validation Plan

## Overview

All tests run without network access to real registries. CI uses recorded fixtures and mock servers. The only exception is the `record` build tag for capturing new fixtures from live RDAP servers (run manually, not in CI).

## 1. Domain Input Validation Test Cases

### Valid Inputs (should pass validation)

| # | Input | Normalized Output | Notes |
|---|-------|-------------------|-------|
| 1 | `example.com` | `example.com` | Basic domain |
| 2 | `EXAMPLE.COM` | `example.com` | Case normalization |
| 3 | `Example.Com` | `example.com` | Mixed case |
| 4 | `sub.example.com` | `example.com` | Subdomain stripped to eTLD+1 |
| 5 | `deep.sub.example.com` | `example.com` | Multi-level subdomain |
| 6 | `example.co.uk` | `example.co.uk` | Multi-level TLD |
| 7 | `www.example.co.uk` | `example.co.uk` | Subdomain + multi-level TLD |
| 8 | `example.com.au` | `example.com.au` | Multi-level TLD (Australia) |
| 9 | `example.com.` | `example.com` | Trailing dot (FQDN notation) |
| 10 | `my-domain.com` | `my-domain.com` | Hyphen in label |
| 11 | `a.com` | `a.com` | Single-char label |
| 12 | `1example.com` | `1example.com` | Label starts with digit (RFC 1123) |
| 13 | `123.com` | `123.com` | All-numeric label |
| 14 | `a-b-c.com` | `a-b-c.com` | Multiple hyphens |
| 15 | `xn--nxasmq6b.com` | `xn--nxasmq6b.com` | Punycode domain |
| 16 | `münchen.de` | `xn--mnchen-3ya.de` | IDN → punycode conversion |
| 17 | `例え.jp` | `xn--r8jz45g.jp` | CJK IDN |
| 18 | `  example.com  ` | `example.com` | Whitespace trimmed |
| 19 | `a`+`a`×62+`.com` | 63-char label `.com` | Max label length (63) |
| 20 | `example.dev` | `example.dev` | Newer gTLD |
| 21 | `example.io` | `example.io` | ccTLD popular with tech |
| 22 | `example.xyz` | `example.xyz` | New gTLD |

### Invalid Inputs (should fail validation with specific error)

| # | Input | Expected Error | Rule Violated |
|---|-------|---------------|---------------|
| 23 | (empty string) | `empty domain` | Minimum length |
| 24 | `com` | `single label` | Must have ≥ 2 labels |
| 25 | `.com` | `empty label` | Leading dot = empty first label |
| 26 | `example..com` | `empty label` | Consecutive dots |
| 27 | `.` | `empty domain` | Dot only |
| 28 | `-example.com` | `label starts with hyphen` | LDH rule |
| 29 | `example-.com` | `label ends with hyphen` | LDH rule |
| 30 | `exam_ple.com` | `invalid character: _` | LDH rule (underscore) |
| 31 | `exam ple.com` | `invalid character: space` | LDH rule |
| 32 | `exam<ple.com` | `invalid character: <` | LDH rule |
| 33 | `example.com/path` | `invalid character: /` | URL fragment rejection |
| 34 | `http://example.com` | `invalid character: :` | URL scheme rejection |
| 35 | `user@example.com` | `invalid character: @` | Email format rejection |
| 36 | `example.com:8080` | `invalid character: :` | Port rejection |
| 37 | 64×`a`+`.com` | `label exceeds 63 characters` | Max label length |
| 38 | 254-char domain | `domain exceeds 253 characters` | Max total length |
| 39 | `ab--cd.com` | `hyphens at positions 3-4` | RFC 5891 §4.2.3.1 (non-xn--) |
| 40 | `exam\x00ple.com` | `invalid character: null byte` | Null byte injection |
| 41 | `example.invalidtld` | `unsupported TLD` | TLD not in bootstrap or WHOIS list |
| 42 | `🎉.com` | `invalid character` | Emoji not valid in domain labels |
| 43 | `example` | `single label` | No TLD |
| 44 | `localhost` | `single label` | Reserved name, no TLD |

### Edge Cases (behavior may vary — document and test the choice)

| # | Input | Decision | Rationale |
|---|-------|----------|-----------|
| 45 | `example.github.io` | Reject or check `github.io`? | PSL PRIVATE section — use `IgnorePrivate` to skip, treat as `github.io` not being a registrable suffix. Return error: "cannot check availability under private suffix." |
| 46 | `*.example.com` | Reject: `invalid character: *` | Wildcard not a valid domain |
| 47 | `xn--invalid.com` | Pass validation, let RDAP determine | Invalid punycode — RDAP will return 404 or 400 |
| 48 | `test.com` (reserved) | Pass validation, check normally | Reservation is a registry concern, not input validation |

## 2. RDAP Response Fixture Inventory

### Registered Domain Fixtures (HTTP 200)

| Fixture File | Registry | TLD | Key Edge Cases Covered |
|-------------|----------|-----|----------------------|
| `verisign-google.com.json` | Verisign | .com | UPPERCASE `ldhName`, no fractional seconds in dates |
| `pir-wikipedia.org.json` | PIR | .org | `redacted` array (RFC 9537), millisecond dates |
| `google-web.dev.json` | Google | .dev | `reregistration` event type, `zoneSigned` in secureDNS |
| `centralnic-abc.xyz.json` | CentralNic | .xyz | `port43` field, `lang` field, 1-digit fractional second |
| `identity-digital-nic.live.json` | Identity Digital | .live | Self-referencing `related` link, `redacted` array |
| `nominet-bbc.uk.json` | Nominet | .uk | Trailing dot on nameservers, microsecond dates |
| `denic-example.de.json` | DENIC | .de | Minimal response (no links, no secureDNS, custom conformance) |
| `registro-br-example.br.json` | Registro.br | .br | `legalRepresentative`, CNPJ publicIds, custom events |

### Available Domain Fixtures (HTTP 404)

| Fixture File | Registry | Body |
|-------------|----------|------|
| `verisign-404.txt` | Verisign | Empty (0 bytes) |
| `pir-404.json` | PIR | JSON with errorCode, title, description |
| `google-404.json` | Google | JSON with errorCode, description |
| `centralnic-404.json` | CentralNic | JSON with `objectClassName: "error"` |

### Error Fixtures

| Fixture File | Scenario | HTTP Code |
|-------------|----------|-----------|
| `google-429.json` | Rate limited | 429 |
| `pir-400.json` | Invalid domain format | 400 |
| `verisign-400-empty.txt` | Invalid format (Verisign returns 404) | 404 |
| `timeout.txt` | Simulated timeout | N/A (context deadline exceeded) |
| `connection-refused.txt` | Unreachable registry | N/A (dial error) |

### WHOIS Fixtures (for ccTLD fallback)

| Fixture File | Registry | Scenario |
|-------------|----------|----------|
| `whois-de-registered.txt` | DENIC (.de) | Registered domain |
| `whois-de-available.txt` | DENIC (.de) | Available domain |
| `whois-jp-registered.txt` | JPRS (.jp) | Registered domain |
| `whois-jp-available.txt` | JPRS (.jp) | Available domain |

## 3. RDAP Response Parser Tests

| # | Test Name | Input | Expected Behavior |
|---|-----------|-------|-------------------|
| 1 | Parse Verisign registered | `verisign-google.com.json` | Extract registrar, dates, nameservers; normalize `ldhName` to lowercase |
| 2 | Parse PIR registered | `pir-wikipedia.org.json` | Handle `redacted` array, millisecond dates |
| 3 | Parse Google registered | `google-web.dev.json` | Handle `reregistration` event without error |
| 4 | Parse DENIC minimal | `denic-example.de.json` | Handle missing `links`, `secureDNS`, empty `entities` |
| 5 | Parse Nominet registered | `nominet-bbc.uk.json` | Strip trailing dots from nameserver names |
| 6 | Parse Registro.br | `registro-br-example.br.json` | Ignore non-standard fields without error |
| 7 | Parse Verisign 404 (empty) | Empty bytes | Return `available: true`, no JSON parse error |
| 8 | Parse PIR 404 (JSON error) | `pir-404.json` | Return `available: true` |
| 9 | Parse 429 rate limit | `google-429.json` | Return rate-limit error, not `available: true` |
| 10 | Parse connection error | Simulated dial failure | Return connection error, not `available: true` |
| 11 | Parse timezone offset date | `2018-03-12T21:44:25+01:00` | Parse correctly (DENIC format) |
| 12 | Parse zero-fraction date | `2014-03-20T12:59:17.0Z` | Parse correctly (CentralNic) |
| 13 | Parse microsecond date | `2025-10-29T03:51:11.009091Z` | Parse correctly (Nominet) |
| 14 | Parse +00:00 date | `2028-09-14T07:00:00.000+00:00` | Parse correctly (MarkMonitor) |
| 15 | Unknown event type | `{"eventAction": "delegation check"}` | Ignore without error |
| 16 | Missing handle field | Response without `handle` | Parse successfully, `handle` is empty string |
| 17 | CentralNic error object | `{"objectClassName": "error"}` | Don't treat as domain object |

## 4. API Integration Tests

### Single Check Endpoint (`GET /api/v1/check`)

| # | Request | Expected Response |
|---|---------|-------------------|
| 1 | `?d=available.com` | 200, `available: true`, source: "rdap" |
| 2 | `?d=taken.com` | 200, `available: false`, registrar/dates populated |
| 3 | `?d=` (empty) | 400, `error: invalid_domain` |
| 4 | `?d=not a domain` | 400, `error: invalid_domain` |
| 5 | `?d=example.invalidtld` | 400, `error: unsupported_tld` |
| 6 | (no `d` param) | 400, `error: missing parameter "d"` |
| 7 | `?d=cached.com` (repeat) | 200, `cached: true` on second request |

### Multi-TLD Endpoint (`GET /api/v1/check?tlds=`)

| # | Request | Expected |
|---|---------|----------|
| 8 | `?d=example&tlds=com,org` | 200, results array with 2 entries |
| 9 | `?d=example&tlds=com,invalidtld` | 200, partial results (com succeeds, invalidtld has error) |
| 10 | `?d=example` (no tlds) | Single check on `example` as-is |

### Bulk Endpoint (`POST /api/v1/bulk`)

| # | Request | Expected |
|---|---------|----------|
| 11 | 3 valid domains | 200, 3 results |
| 12 | 50 domains (max) | 200, 50 results |
| 13 | 51 domains (over limit) | 400, `error: max 50 domains per request` |
| 14 | Empty array | 400, `error: no domains provided` |
| 15 | 128 KB body | 413, body too large |
| 16 | Invalid JSON | 400, `error: invalid JSON` |
| 17 | Mixed success/failure | 200, per-domain results with individual errors |

### Rate Limiting

| # | Scenario | Expected |
|---|----------|----------|
| 18 | 11th web check in 1 minute from same IP | 429 with `retry_after` |
| 19 | 61st API check in 1 minute from same IP | 429 with `retry_after` |
| 20 | Different IPs within limits | All succeed |
| 21 | Rate limit response includes `Retry-After` header | Header present |

### Security

| # | Scenario | Expected |
|---|----------|----------|
| 22 | Response has `Content-Security-Policy` header | Present on all responses |
| 23 | Response has `X-Content-Type-Options: nosniff` | Present |
| 24 | Response has `X-Frame-Options: DENY` | Present |
| 25 | Response has `X-Request-Id` header | Present, 16 hex chars |
| 26 | CORS preflight `OPTIONS /api/v1/check` | 204, correct `Access-Control-*` headers |
| 27 | API response `Content-Type` | `application/json` |

### Health Check

| # | Scenario | Expected |
|---|----------|----------|
| 28 | `GET /health` normal | 200, `status: ok`, bootstrap age < 48h |
| 29 | Bootstrap older than 48h | 200, `status: degraded` |
| 30 | Bootstrap failed to load | 503, `status: unhealthy` |

## 5. Web UI Tests (Playwright or similar)

| # | Scenario | Assertion |
|---|----------|-----------|
| 1 | Load home page | Search input visible, form action is `/check` |
| 2 | Submit empty form | Error message displayed |
| 3 | Submit valid domain (no JS) | Redirect to `/check?d=...`, result displayed |
| 4 | Submit valid domain (with JS) | Result appears inline without page reload |
| 5 | Check available domain | Green "Available" indicator |
| 6 | Check taken domain | Red "Taken" indicator, registration details visible |
| 7 | Multi-TLD checkboxes | Checking .com + .org returns results for both |
| 8 | Mobile viewport (390×844) | Layout is usable, input is full-width |
| 9 | Result page is shareable | URL contains domain parameter, reloading shows same result |

## 6. Fuzz Testing

### Target: `ValidateDomain()`

```go
func FuzzValidateDomain(f *testing.F) {
    // Seed corpus
    f.Add("example.com")
    f.Add("")
    f.Add(".")
    f.Add(strings.Repeat("a", 254))
    f.Add("xn--nxasmq6b.com")
    f.Add("-leading.com")
    f.Add("trailing-.com")
    f.Add("\x00.com")
    f.Add("example..com")

    f.Fuzz(func(t *testing.T, domain string) {
        err := ValidateDomain(domain)
        // Property: never panics (implicit)
        if err == nil {
            // Property: valid domains have at least one dot
            // Property: no label exceeds 63 chars
            // Property: total length ≤ 253
        }
    })
}
```

### Target: `ParseRDAPResponse()`

```go
func FuzzParseRDAPResponse(f *testing.F) {
    // Seed with real fixture files
    for _, fixture := range fixtureFiles {
        f.Add(readFixture(fixture))
    }

    f.Fuzz(func(t *testing.T, data []byte) {
        result, err := ParseRDAPResponse(data)
        // Property: never panics
        // Property: if no error, result.Domain is non-empty
        // Property: if no error, result.Available is deterministic for same input
    })
}
```

CI runs fuzz tests for 30 seconds. Crash cases auto-saved to `testdata/fuzz/` and become permanent regression tests.

## 7. Load Testing

### Targets

| Scenario | Target p99 | Target Error Rate |
|----------|-----------|-------------------|
| Cached responses | < 10ms | < 0.1% |
| Uncached single check | < 2s | < 1% |
| Bulk (50 domains, mixed TLDs) | < 5s | < 2% |
| Sustained 100 req/s (cached) | < 50ms | < 0.1% |

### Test Commands

```bash
# Quick smoke test
hey -n 1000 -c 50 "http://localhost:8080/api/v1/check?d=example.com"

# Sustained rate test
echo "GET http://localhost:8080/api/v1/check?d=example.com" | \
  vegeta attack -rate=100/s -duration=60s | vegeta report

# Rate limiter verification
echo "GET http://localhost:8080/api/v1/check?d=example.com" | \
  vegeta attack -rate=200/s -duration=10s | vegeta report
# Should show 429 responses after rate limit exceeded
```

### Memory Growth Test

Run `vegeta` at 50 req/s for 10 minutes from randomized IPs. Monitor memory via `/metrics` — should plateau (LRU eviction + rate limiter cleanup working). Memory growth >100 MB indicates a leak.

## 8. CI Pipeline

```yaml
name: CI
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        go: ['1.22', '1.23']
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ matrix.go }}

      - name: Lint
        uses: golangci/golangci-lint-action@v6

      - name: Test
        run: go test -race -coverprofile=coverage.out ./...

      - name: Fuzz (30s)
        run: |
          go test -fuzz=FuzzValidateDomain -fuzztime=30s ./internal/domain/
          go test -fuzz=FuzzParseRDAPResponse -fuzztime=30s ./internal/checker/

      - name: Build
        run: go build -o domain-check ./cmd/domain-check/

      - name: Upload coverage
        uses: codecov/codecov-action@v4
```

### CI Guarantees

- Tests run with `-race` flag (data race detector)
- Two Go versions tested (current + previous)
- Fuzz tests run for 30s on each push
- golangci-lint catches common issues
- Coverage reported to Codecov
- **No network calls to real RDAP servers in CI** — all tests use fixtures

## 9. Regression Process

When a bug is found:

1. **Write a failing test first** that reproduces the bug
2. If it's an input validation bug → add to the table-driven test list above
3. If it's an RDAP parsing bug → add a new fixture file capturing the problematic response
4. If it's found by fuzzing → the crash case is already saved in `testdata/fuzz/` automatically
5. Fix the bug
6. Verify the test passes
7. The test is now a permanent regression test

Fuzz crash cases in `testdata/fuzz/` are committed to the repo and run on every `go test` invocation (without the `-fuzz` flag).
