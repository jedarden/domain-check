# bf-12ek: Per-registry HTTP connection pool tuning

Task: Add HTTP connection pooling tuned per registry.

## Status: Already implemented in commit d2ff495

The implementation was completed in `feat: add per-registry HTTP connection pool tuning` and is fully functional.

## What was implemented

`internal/checker/ssrf.go`:
- `PoolConfig` struct: holds `MaxIdleConns` and `MaxConns` per registry
- `registryPoolConfigs` map: known registries with pools sized to their concurrency limits
  - `rdap.verisign.com`: MaxIdleConns=10, MaxConns=12 (10 RPS, concurrency 10)
  - `rdap.publicinterestregistry.org`: MaxIdleConns=10, MaxConns=12 (10 RPS, concurrency 10)
  - `pubapi.registry.google`: MaxIdleConns=2, MaxConns=3 (1 RPS, concurrency 2)
- `defaultPoolConfig`: MaxIdleConns=3, MaxConns=5 for unknown registries
- `newTransportForPool`: creates an `*http.Transport` from a `PoolConfig`
- `PerRegistryRoundTripper`: dispatches each request to the per-host transport, falling back to the default
- `NewPerRegistryRoundTripper()`: constructor wiring all transports with the shared `SafeDialer`

`cmd/domain-check/main.go`:
- `setupDomainChecker` passes `checker.NewPerRegistryRoundTripper()` as the transport to `NewSafeClient`

## Tests (all passing)

- `TestPerRegistryRoundTripper_PoolSizes`: verifies each known registry has a dedicated transport with correct pool sizes
- `TestPerRegistryRoundTripper_Routing`: verifies requests are dispatched to the correct transport by hostname
- `TestNewSafeClient_CustomTransport`: verifies `ClientConfig.Transport` override works
- `TestRegistryPoolConfigs_AlignWithRateLimits`: enforces that `MaxConns >= Concurrency` for all known registries
