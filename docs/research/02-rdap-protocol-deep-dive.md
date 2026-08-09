# RDAP Protocol Deep Dive

> Technical details of the Registration Data Access Protocol for domain availability checking.

## Protocol Overview

RDAP (RFC 7480-7484, RFC 9083) is the IETF-standardized replacement for WHOIS. It uses HTTPS and returns structured JSON, eliminating the parsing nightmares of the legacy WHOIS text protocol.

## Bootstrap Process

IANA maintains a machine-readable mapping of TLDs to their authoritative RDAP servers:

```
https://data.iana.org/rdap/dns.json
```

As of 2026-03-17, this file contains **591 TLD mappings**. Structure:

```json
{
  "version": "1.0",
  "publication": "2026-03-17T18:19:24Z",
  "services": [
    [
      ["com"],
      ["https://rdap.verisign.com/com/v1/"]
    ],
    [
      ["ads", "android", "app", "boo", "cal", "channel", "chrome", "dad", "day", "dev", ...],
      ["https://pubapi.registry.google/rdap/"]
    ],
    [
      ["charity", "foundation", "gives", "giving", "ngo", "ong", "org", ...],
      ["https://rdap.publicinterestregistry.org/rdap/"]
    ]
  ]
}
```

Each entry maps one or more TLDs to one or more RDAP server URLs.

## Query Format

```
GET {rdap_server}/domain/{domain_name}
```

Examples:
```
GET https://rdap.verisign.com/com/v1/domain/google.com
GET https://rdap.publicinterestregistry.org/rdap/domain/example.org
GET https://pubapi.registry.google/rdap/domain/example.dev
```

No authentication headers, no API keys, no request body.

## Response Format

### Registered Domain (HTTP 200)

```json
{
  "objectClassName": "domain",
  "handle": "2138514_DOMAIN_COM-VRSN",
  "ldhName": "GOOGLE.COM",
  "status": [
    "client delete prohibited",
    "client transfer prohibited",
    "server delete prohibited",
    "server transfer prohibited"
  ],
  "events": [
    { "eventAction": "registration", "eventDate": "1997-09-15T04:00:00Z" },
    { "eventAction": "expiration", "eventDate": "2028-09-14T04:00:00Z" },
    { "eventAction": "last changed", "eventDate": "2019-09-09T15:39:04Z" }
  ],
  "entities": [
    {
      "roles": ["registrar"],
      "vcardArray": ["vcard", [["fn", {}, "text", "MarkMonitor Inc."]]]
    }
  ],
  "nameservers": [
    { "ldhName": "NS1.GOOGLE.COM" },
    { "ldhName": "NS2.GOOGLE.COM" }
  ]
}
```

### Unregistered Domain (HTTP 404)

Empty response body. The HTTP status code alone is the signal.

### Rate Limited (HTTP 429)

Google's registry returns 429 after ~10 rapid queries. Recovers in ~5 seconds.

## Key Registries and Their RDAP Servers

| Registry Operator | TLDs | RDAP Server | Notes |
|-------------------|------|-------------|-------|
| Verisign | .com, .net, .cc, .tv, .name | `https://rdap.verisign.com/{tld}/v1/` | Most permissive rate limits |
| Public Interest Registry | .org, .ngo, .ong, .charity, .foundation, .gives, .giving | `https://rdap.publicinterestregistry.org/rdap/` | Generous rate limits |
| Google Registry | .dev, .app, .page, .how, .soy, .new, .day, .dad, .phd, .prof, .mov, .zip, .foo, .nexus, .ing, .meme, .channel, .chrome, .android, .ads, .cal, .boo + more | `https://pubapi.registry.google/rdap/` | Rate limits at ~10/burst, recovers in 5s |
| CentralNic | .xyz, .online, .site, .website, .store, .tech, .space, .fun, .host, .press + many more | `https://rdap.centralnic.com/{tld}/` | |
| Donuts / Identity Digital | .live, .world, .life, .today, .email, .solutions, .agency, .studio + hundreds more | `https://rdap.identitydigital.services/rdap/` | |
| Cloudflare | — | `https://rdap.cloudflare.com/` | Registrar-level, not registry |

## Proxy Services

**rdap.org** — A free bootstrap proxy. It reads the IANA bootstrap file and redirects queries to the correct registry.

```
GET https://rdap.org/domain/example.com
→ 302 Redirect to https://rdap.verisign.com/com/v1/domain/example.com
```

Rate limited by Cloudflare: ~10 requests per 10 seconds. Useful for mixed-TLD queries when you don't want to maintain your own bootstrap routing.

## Why RDAP Is Free

Registries (Verisign, PIR, Google, etc.) are contractually required by ICANN to provide RDAP access as a condition of operating a TLD. The cost is absorbed into wholesale domain registration fees (e.g., $7.85/year for .com goes to Verisign). This is analogous to how telephone companies provide directory assistance — it's a regulatory obligation, not a commercial product.

Paid RDAP services (rdapapi.io, WhoisFreaks, etc.) add convenience layers (bulk endpoints, caching, uptime SLAs, unified response formatting) on top of the free registry infrastructure.

## Limitations

1. **Cannot detect reserved names** — some domains are reserved by registries and cannot be registered, but RDAP returns 404 (looks available)
2. **Cannot detect premium pricing** — a domain may be available but cost $5,000+; RDAP doesn't include pricing
3. **Redemption period** — recently expired domains in the ~30-day grace period are technically registered but may appear differently
4. **ccTLD coverage** — most ccTLDs now have RDAP, but some legacy ccTLDs may not be in the bootstrap file
5. **Rate limits** — undocumented per-registry; must be discovered empirically

## Implementation Notes

- Cache the IANA bootstrap file locally (update daily or weekly)
- Map TLDs to RDAP servers at startup
- Use RDAP direct to registries (not rdap.org proxy) for higher throughput
- Respect per-registry rate limits (Verisign is generous; Google is strict)
- Fall back to WHOIS for TLDs not in the bootstrap file
- HTTP 200 = taken, HTTP 404 = available, HTTP 429 = rate limited (retry after delay)
