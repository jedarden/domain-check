# bf-u4cb: Add fuzz crash corpus to testdata/fuzz/

Task: Run fuzz tests and commit discovered corpus entries as permanent regression tests.

## Status: Complete

Fuzz corpus was previously established in commit `7a6124a` (891 FuzzValidateDomain entries,
577 FuzzParseRDAPResponse entries). This execution ran extended fuzz sessions to discover new
inputs and committed 3 additional FuzzParseRDAPResponse entries.

## Fuzz run results (2026-05-15)

### FuzzValidateDomain (5 minutes)
- Executions: 36,456,761 (~121k/sec)
- New interesting inputs: 0
- Total corpus: 901

### FuzzParseRDAPResponse (5 minutes)
- Executions: 16,064,159 (~53k/sec)
- New interesting inputs: 3
- Total corpus: 592

## New corpus entries (committed in 41cc1a6)

Three new FuzzParseRDAPResponse entries were discovered with malformed UTF-8 sequences
(`\x8b`, `\xf1`) embedded in otherwise valid RDAP JSON — edge cases the parser handles
gracefully without panicking.

- `internal/checker/testdata/fuzz/FuzzParseRDAPResponse/8b6532fc007efef7`
- `internal/checker/testdata/fuzz/FuzzParseRDAPResponse/b168fcc4fecf5718`
- `internal/checker/testdata/fuzz/FuzzParseRDAPResponse/d040457b8c36ca30`

## Corpus layout

Per Go conventions, seed corpus lives alongside each package's test files:
- `internal/domain/testdata/fuzz/FuzzValidateDomain/` — 892 entries
- `internal/checker/testdata/fuzz/FuzzParseRDAPResponse/` — 580 entries
