# ccTLD Handling and Public Suffix List

## The Multi-Level TLD Problem

You cannot split on the first dot to extract a TLD. `example.co.uk` has TLD `co.uk`, not `uk`. The Public Suffix List (PSL) is the only reliable solution.

## Public Suffix List (PSL)

Maintained by Mozilla at `https://publicsuffix.org/`. Community-driven via `github.com/publicsuffix/list`.

- **Size**: ~9,000 rules (~250-300 KB raw file)
- **Sections**: ICANN DOMAINS (official TLDs, ~2,500 rules) and PRIVATE DOMAINS (github.io, appspot.com, etc., ~6,500 rules)
- **Update frequency**: Roughly weekly
- **Format rules**:
  - One rule per line, comments start with `//`
  - Wildcard: `*.ck` means all second-level domains under `.ck` are public suffixes
  - Exception: `!www.ck` overrides a wildcard

## Go Libraries

### `golang.org/x/net/publicsuffix` (standard)
- Embeds a compiled PSL snapshot at build time
- 3,417+ downstream imports — de facto standard
- Key API:
  - `PublicSuffix("example.co.uk")` → `"co.uk", true`
  - `EffectiveTLDPlusOne("www.books.amazon.co.uk")` → `"amazon.co.uk"`
- **Limitation**: PSL data is frozen at dependency version; does not auto-update at runtime

### `github.com/weppos/publicsuffix-go` (advanced)
- Supports runtime list loading
- `Parse("www.example.co.uk")` → `{Suffix: "co.uk", Domain: "example", Subdomain: "www"}`
- `IgnorePrivate` option — skip PRIVATE section (recommended for domain availability checking)
- Drop-in compatibility adapter available

**Recommendation**: Use `golang.org/x/net/publicsuffix` for simplicity. Only need `IgnorePrivate`? Use `weppos/publicsuffix-go`.

## ccTLD RDAP Coverage

### Have RDAP (in IANA bootstrap)

| ccTLD | RDAP Server | Operator |
|-------|-------------|----------|
| .io | `https://rdap.nic.io/` | Identity Digital |
| .me | `https://rdap.nic.me/` | Afilias |
| .tv | `https://rdap.nic.tv/` | Verisign |
| .ai | `https://rdap.nic.ai/` | Offshore Registrars |
| .ly | `https://rdap.nic.ly/` | ARIN |
| .fm | `https://rdap.centralnic.com/fm/` | CentralNic |
| .to | `https://rdap.tonicregistry.to/rdap/` | Tonic |
| .cc | `https://tld-rdap.verisign.com/cc/v1/` | Verisign |
| .uk | `https://rdap.nominet.uk/uk/` | Nominet |
| .fr | `https://rdap.nic.fr/` | AFNIC |
| .nl | `https://rdap.sidn.nl/` | SIDN |
| .ca | `https://rdap.ca.fury.ca/rdap/` | CIRA |
| .au | `https://rdap.cctld.au/rdap/` | auDA |
| .in | `https://rdap.nixiregistry.in/rdap/` | NIXI |
| .br | `https://rdap.registro.br/` | NIC.br |

### NO RDAP — WHOIS Fallback Required

| ccTLD | WHOIS Server | Notes |
|-------|-------------|-------|
| .de | `whois.denic.de` | Largest ccTLD by registrations. Aggressive rate limiting. |
| .co | `whois.nic.co` | Popular alternative TLD |
| .gg | `whois.gg` | Gaming/tech niche |
| .se | `whois.iis.se` | Sweden |
| .ch | `whois.nic.ch` | Switzerland |
| .at | `whois.nic.at` | Austria |
| .be | `whois.dns.be` | Belgium |
| .nz | `whois.srs.net.nz` | New Zealand |
| .jp | `whois.jprs.jp` | Japan |
| .kr | `whois.kr` | South Korea |
| .cn | `whois.cnnic.cn` | China |
| .ru | `whois.tcinet.ru` | Russia |

**Statistics**: ~314M domains under TLDs with RDAP. ~45M under TLDs with no RDAP.

## WHOIS Fallback Libraries (Go)

- **`github.com/likexian/whois`** — automatic server discovery, referral chain following, configurable timeouts
- **`github.com/likexian/whois-parser`** — companion for extracting structured fields from raw WHOIS text
- **`github.com/domainr/whois`** — per-TLD adapter system, battle-tested at Domainr

## IDN / Punycode

RDAP requires A-labels (punycode) in lowercase per RFC 9224.

**Go package**: `golang.org/x/net/idna`

```go
// Convert Unicode domain to ASCII for RDAP query
ascii, err := idna.Lookup.ToASCII("münchen.de")
// ascii = "xn--mnchen-3ya.de"

// Convert back for display
unicode, err := idna.Lookup.ToUnicode("xn--mnchen-3ya.de")
```

Profiles: `idna.Lookup` (for queries, permissive), `idna.Registration` (for registration, strict), `idna.Display` (for showing to users).

IDN domains are ~5-10M out of ~350M+ total registrations — small fraction but important in specific markets (.cn, .de, .jp, .ru).

## Bootstrap-to-PSL Lookup Flow

When processing `mysite.co.uk`:
1. `PublicSuffix("mysite.co.uk")` → `"co.uk"` (the eTLD)
2. `EffectiveTLDPlusOne("mysite.co.uk")` → `"mysite.co.uk"` (the registrable domain to query)
3. Extract rightmost label: `"uk"` (for IANA bootstrap lookup)
4. Bootstrap lookup `"uk"` → `https://rdap.nominet.uk/uk/`
5. Query: `GET https://rdap.nominet.uk/uk/domain/mysite.co.uk`
