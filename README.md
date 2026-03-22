# Domain Check

Authoritative domain availability checker powered by RDAP — the ICANN-mandated successor to WHOIS.

## Features

- **Authoritative** — queries registry RDAP servers directly (Verisign, PIR, Google, etc.)
- **Web UI** — clean, fast, no-signup interface for humans
- **REST API** — JSON API for programmatic/machine consumption
- **Bulk checking** — check up to 50 domains in a single request
- **Multi-TLD** — check a name across .com, .org, .dev, .net, .io, .app at once
- **Self-hostable** — single Go binary or Docker container
- **Zero tracking** — no analytics, no cookies, no data retention
- **Open source** — MIT license

## Why RDAP over WHOIS?

| | RDAP | WHOIS |
|---|---|---|
| Protocol | HTTPS (JSON) | TCP port 43 (plaintext) |
| Accuracy | Definitive | Unreliable (missed registered domains in testing) |
| .dev/.app support | Yes | No (Google disabled port 43) |
| Response format | Structured JSON | Unstructured text (varies by registry) |
| Parsing | Trivial | Fragile regex per-registry |

In our testing, `whois` reported a registered domain as available. RDAP was 100% accurate across all test cases. See [docs/research/05-accuracy-comparison.md](docs/research/05-accuracy-comparison.md).

## Documentation

- [docs/research/](docs/research/) — Research on domain checking methods, RDAP protocol, rate limits, accuracy testing
- [docs/plan/plan.md](docs/plan/plan.md) — Complete architecture plan for the web UI and API

## License

MIT
