# Domain Availability Checking Methods

> Research comparing all practical approaches for programmatically determining if a domain name is registered.

## Methods Evaluated

### 1. RDAP (Registration Data Access Protocol) — RECOMMENDED

RDAP is the ICANN-mandated successor to WHOIS. It works over HTTPS and returns structured JSON.

**How it works:**
1. Download the IANA bootstrap file: `https://data.iana.org/rdap/dns.json`
2. Look up which RDAP server handles the target TLD
3. Send an HTTPS GET to `{rdap_server}/domain/{domain_name}`
4. Check the HTTP status code:
   - `200` = domain is registered (response includes registrar, dates, nameservers)
   - `404` = domain is not in the registry (available)

**Key RDAP servers:**
| TLD | RDAP Server |
|-----|-------------|
| .com | `https://rdap.verisign.com/com/v1/domain/{domain}` |
| .net | `https://rdap.verisign.com/net/v1/domain/{domain}` |
| .org | `https://rdap.publicinterestregistry.org/rdap/domain/{domain}` |
| .dev, .app, .page | `https://pubapi.registry.google/rdap/domain/{domain}` |
| .xyz | `https://rdap.centralnic.com/xyz/domain/{domain}` |
| .io | Via IANA bootstrap lookup |

**Measured rate limits (from our testing on 2026-03-22):**
| Registry | Burst Tolerance | Recovery | Safe Sustained Rate |
|----------|----------------|----------|-------------------|
| Verisign (.com/.net) | 50+ concurrent, no throttle | N/A | 10+/sec |
| PIR (.org) | 20+ sequential, no throttle | N/A | 10+/sec |
| Google (.dev/.app) | ~10 rapid, then HTTP 429 | 5 seconds | 1/sec |
| rdap.org proxy | ~10 per 10 sec | ~10 sec | 1/sec |

**Pros:**
- Free, no API key, no account required
- Machine-readable JSON responses (RFC 9083)
- Definitive answer — queries the authoritative registry directly
- Works for ALL gTLDs including .dev, .app (which have no WHOIS)
- HTTPS-based (encrypted, cacheable)
- 591 TLD mappings in the bootstrap file

**Cons:**
- Cannot detect reserved names, premium pricing, or redemption-period domains
- Some ccTLDs may lack RDAP servers (not contractually required by ICANN)
- Registry-specific rate limits are undocumented (must be tested)

### 2. WHOIS (Port 43) — LEGACY, UNRELIABLE

Traditional WHOIS sends plaintext queries to port 43 of registry servers.

```bash
whois example.com | grep -i "No match"
```

**Pros:** Pre-installed on most Linux systems, works for legacy TLDs.

**Cons:**
- **Unreliable results** — in our testing, `whois` reported `numcrunch.com` as available when it was actually registered (RDAP confirmed taken)
- Unstructured text output varies wildly by TLD/registry — requires per-registry regex parsing
- **Does not work for .dev, .app, .page** — Google disabled port 43 WHOIS for 40+ TLDs
- Rate limits: ~5/min safe for Verisign, various registries block after sustained use
- No standardized response format

**Verdict:** Do not use as primary method. RDAP supersedes it in every way.

### 3. DNS (NS Record Check) — FAST BUT INCOMPLETE

Check if TLD nameservers have NS delegation records for the domain.

```bash
dig +short NS example.com
# Records returned = definitely registered
# NXDOMAIN = *probably* not registered (but NOT definitive)
```

**Measured accuracy (2026-03-22):**
| Domain | RDAP (truth) | DNS result | Correct? |
|--------|-------------|-----------|----------|
| abacash.com | TAKEN | NS records found | Yes |
| compoundly.com | TAKEN | NS records found | Yes |
| numcrunch.com | TAKEN | **NXDOMAIN** | **NO — false available** |
| dimecalc.com | AVAILABLE | NXDOMAIN | Yes |
| publiccalc.com | AVAILABLE | NXDOMAIN | Yes |

**Pros:** Fastest method (~0.1s), unlimited queries, no rate limits.

**Cons:**
- **False negatives** — registered domains with no DNS configured (parked, unused) return NXDOMAIN. `numcrunch.com` was registered but showed NXDOMAIN.
- Cannot be trusted alone for availability determination.

**Verdict:** Useful only as a fast pre-filter. If NS records exist → definitely taken (skip RDAP). If NXDOMAIN → must verify with RDAP.

### 4. Certificate Transparency Logs — SUPPLEMENTARY SIGNAL

Query crt.sh (or other CT log aggregators) for certificates issued to a domain.

```bash
curl -s "https://crt.sh/?q=example.com&output=json" | python3 -c "import sys,json; print(len(json.load(sys.stdin)))"
```

**Measured accuracy (2026-03-22):**
| Domain | RDAP (truth) | CT certs found | Correct? |
|--------|-------------|---------------|----------|
| abacash.com | TAKEN | 39 certs | Yes |
| napkinmath.com | TAKEN | 96 certs | Yes |
| numcrunch.com | TAKEN | 8 certs | Yes (caught what DNS missed) |
| mathpact.com | AVAILABLE | **4 certs** | **NO — stale certs from prior registration** |
| dimecalc.com | AVAILABLE | 0 certs | Yes |

**Pros:** Catches registered domains that DNS misses (numcrunch.com). Provides historical registration signal.

**Cons:**
- **False positives** — stale certificates from expired/dropped domains persist in CT logs forever
- **False negatives** — newly registered domains without a website have no certs
- Slower (~1-2s per query)
- crt.sh has its own rate limits and availability issues

**Verdict:** Interesting supplementary signal but not reliable standalone. The stale cert problem (mathpact.com) makes it unsuitable as an authority.

### 5. Registrar APIs — MOST ACCURATE (but requires accounts)

Query registrar APIs (GoDaddy, Namecheap, Porkbun, Cloudflare) for availability.

| Registrar | Rate Limit | Requirement |
|-----------|-----------|-------------|
| Porkbun | Dynamic | Free account, API key |
| Namecheap | 20/min, 700/hr, 8K/day | 20+ domains OR $50 spent |
| GoDaddy | 20K/month | 50+ domains OR $20/mo spend |
| Cloudflare | Unspecified | Free account |

**Pros:** Most accurate — accounts for reserved names, premium pricing, redemption periods.

**Cons:** Requires accounts, API keys, and often minimum spending thresholds.

**Verdict:** Best accuracy but adds operational complexity. RDAP is sufficient for the vast majority of cases.

## Recommendation

**Primary method:** RDAP direct to registry servers
**Pre-filter (optional):** DNS NS check to skip RDAP for domains with active nameservers
**Fallback:** WHOIS for ccTLDs not yet in the RDAP bootstrap
**Do not use:** DNS or CT logs as sole authority

The combination of DNS pre-filter + RDAP authority provides both speed and accuracy with zero cost and no account requirements.
