# Existing Domain Check Tools and Services

> Competitive landscape of domain availability checking tools.

## CLI Tools

### domain-check (Rust, by saidutt46)
- `cargo install domain-check`
- RDAP-first with WHOIS fallback
- 1,200+ TLDs via IANA bootstrap
- Concurrency up to 100, JSON/CSV output
- Presets: startup, tech, creative, finance, ecommerce
- Free, open source

### domaincheck (Rust, by pepa65)
- `cargo install domaincheck`
- RDAP with WHOIS fallback
- Interactive TUI, JSON output
- Free, open source

### rdapcheck (Deno/TypeScript)
- RDAP-only (no WHOIS fallback)
- Pattern support with wildcards
- Configurable concurrency

### whois (system command)
- Pre-installed on Linux
- Port 43 protocol — unreliable for newer TLDs
- Unstructured text output varies by registry

## Web Services

### rdap.org
- Free RDAP bootstrap proxy
- Redirects queries to correct registry
- Cloudflare rate limited: ~10/10sec
- No API key required

### WhoisFreaks
- Free tier: 500 credits, 10 req/min
- Paid: 5,000-150,000 credits ($10-$150)
- 1,500+ TLDs
- REST API with JSON responses

### WhoisXML API
- Free tier: 100 queries
- Credit-based paid tiers
- REST API

### Domainr (by Fastly)
- ICANN-accredited — direct privileged registry access
- API deprecated, now part of Fastly Domain Research API

## Registrar APIs

| Registrar | Free Access | Rate Limit | Requirement |
|-----------|-----------|-----------|-------------|
| Porkbun | Yes | Dynamic | Free account |
| Namecheap | Yes | 20/min, 700/hr, 8K/day | 20+ domains OR $50 spent |
| GoDaddy | Yes | 20K/month | 50+ domains OR $20/mo spend |
| Cloudflare | Yes | Unspecified | Free account |

## Libraries

### Python
| Package | Protocol | Notes |
|---------|----------|-------|
| whodap | RDAP | Best for RDAP, async support |
| whoisit | RDAP | Pure Python, minimal deps |
| python-whois | WHOIS | Most popular but fragile parsing |
| asyncwhois | WHOIS + RDAP | Combined with async |

### Node.js
| Package | Protocol | Notes |
|---------|----------|-------|
| whoiser | WHOIS | Auto-discovers servers via IANA |

## What's Missing

No existing tool combines:
1. **Web UI** for casual human use
2. **REST API** for machine consumption
3. **Authoritative RDAP** (not WHOIS) as the primary protocol
4. **Per-registry rate limiting** with intelligent queuing
5. **Bulk checking** with parallel execution
6. **TLD suggestion** (check a name across multiple TLDs at once)
7. **DNS pre-filter** to skip RDAP for domains with active nameservers
8. **Self-hostable** with zero external dependencies

This is the gap our project fills.
