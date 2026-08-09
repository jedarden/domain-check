# RDAP Response Edge Cases Across Registries

> Based on live queries to 8+ registries on 2026-03-22.

## Schema Differences

| Field | Verisign | PIR | Google | CentralNic | Identity Digital | Nominet | DENIC | Registro.br |
|-------|----------|-----|--------|------------|-----------------|---------|-------|-------------|
| `handle` | Present | Redacted (absent) | Present | Present | Redacted | Present | Absent | Present (=domain) |
| `ldhName` case | **UPPERCASE** | lowercase | lowercase | lowercase | lowercase | lowercase | lowercase | lowercase |
| `unicodeName` | Absent | Present | Absent | Absent | Present | Present | Absent | Absent |
| `links` | Present | Present | Present | Present | Present | Present | **Absent** | Present |
| `secureDNS` | Present | Present | Present | Present | Present | Present | **Absent** | Present |
| `port43` | Absent | Absent | Absent | **Present** | Absent | Absent | Absent | **Present** |
| `redacted` (RFC 9537) | Absent | **Present** | Absent | Absent | **Present** | **Present** | Absent | Absent |

### Critical Parsing Rules

1. **Normalize `ldhName` to lowercase** — Verisign returns UPPERCASE
2. **Never assume a field exists** — use optional access for `handle`, `unicodeName`, `links`, `secureDNS`, `port43`, `redacted`
3. **DENIC is extremely minimal** — no `links`, no `secureDNS`, empty `entities`, custom conformance extension `denic_version_0`
4. **Strip trailing dots from nameserver `ldhName`** — DENIC and Nominet append them (`ns1.google.com.`)
5. **Non-standard fields exist** — Registro.br uses `legalRepresentative`, `publicIds` with type `cnpj`

## Date Format Variations

All ISO 8601 but with significant variation:

| Registry | Example | Notes |
|----------|---------|-------|
| Verisign | `1997-09-15T04:00:00Z` | No fractional seconds |
| PIR | `2012-03-09T16:33:50.754Z` | 3-digit milliseconds |
| CentralNic | `2014-03-20T12:59:17.0Z` | 1-digit fraction |
| Nominet | `2025-10-29T03:51:11.009091Z` | 6-digit microseconds |
| DENIC | `2018-03-12T21:44:25+01:00` | **Timezone offset, not Z** |
| MarkMonitor | `2028-09-14T07:00:00.000+00:00` | `+00:00` instead of `Z` |

**Parser must handle**: 0-6 fractional digits, both `Z` and `±HH:MM` timezone suffixes, non-UTC offsets.

Go's `time.Parse` with `time.RFC3339Nano` handles most of these. For the 1-digit fraction case, may need a fallback parser.

## Event Action Types

Standard: `registration`, `expiration`, `last changed`, `last update of RDAP database`, `transfer`

Non-standard:
- `reregistration` (Google only)
- `registrar expiration` (MarkMonitor registrar-level)
- `delegation check`, `last correct delegation check` (Registro.br)

## Error Responses

### 404 (Not Found — Domain Available)

| Registry | Body |
|----------|------|
| Verisign | **EMPTY (0 bytes)** — curl exit code 56 |
| PIR | JSON: `{errorCode: 404, title: "Object not found", ...}` |
| Google | JSON: `{errorCode: 404, title: "Not Found", ...}` |
| CentralNic | JSON: `{errorCode: 404, objectClassName: "error", ...}` |
| Identity Digital | JSON: `{errorCode: 404, title: "Object not found"}` |

**Critical**: Verisign returns empty body on 404. Parser must handle zero-length responses without attempting JSON parse.

### 400 (Bad Request — Invalid Format)

| Registry | Behavior |
|----------|----------|
| Verisign | Returns **404 (empty body)** — treats invalid as "not found" |
| CentralNic | Returns **404** — same behavior |
| PIR | Returns 400 with JSON error |
| Google | Returns 400 with `description: ["Not a valid domain name"]` |

**Cannot distinguish "invalid format" from "not registered" at Verisign/CentralNic based on status code alone.** Input validation must happen before the RDAP query.

### Other Edge Cases

- **Wrong TLD at registry**: Verisign returns 404 (empty body) for a .org domain queried at the .com endpoint
- **Trailing dot**: Verisign returns 404 (must strip before querying)
- **Subdomain queries** (www.google.com): All registries return 404
- **Overly long label** (>63 chars): Verisign returns 404

## Redirect Behavior

**No registries returned HTTP 301/302/307 redirects.** Instead, they use in-band JSON `links` with `rel: "related"` pointing to registrar RDAP for richer data. This is a second optional request, not an automatic redirect.

## Response Times (from Hetzner Germany)

| Registry | Latency | Protocol |
|----------|---------|----------|
| Verisign (.com) | 80-95ms | HTTP/1.1 |
| CentralNic (.xyz) | 190-200ms | HTTP/2 |
| Google (.dev) | 190-220ms | HTTP/2 |
| Nominet (.uk) | ~280ms | HTTP/2 |
| DENIC (.de) | ~300ms | HTTP/2 |
| PIR (.org) | 540-595ms | HTTP/2 |
| Identity Digital (.live) | 555-655ms | HTTP/2 |
| Registro.br (.br) | ~660ms | HTTP/2 |

**Timeout recommendation**: 5s connect, 10s read, 15s total safety net. Some ccTLD servers (.au, .jp) may be unreachable — handle connection failures as a separate error class.

## CORS

All registries tested return `Access-Control-Allow-Origin: *` — RDAP is designed to be queryable from browsers.

## Content-Type

All return `application/rdap+json` (some add `; charset=utf-8`).

## Caching Headers

Only CentralNic returns cache-friendly headers: `Cache-Control: public, max-age=120` and `ETag`.
