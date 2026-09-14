# Testing and quality strategy

Status: Phase 0 plan

## Quality model

Correctness includes preservation, tenant isolation, bounded memory, cancellation, clear acceptance semantics, stable query meaning, and observable failure—not only happy-path parsing. Tests are deterministic and runnable locally; backend-dependent suites use pinned containers.

## Test layers

### Unit and property tests

- RFC 3164: PRI boundaries, timestamps/year rollover/timezones, host/tag/PID variants, malformed/truncated input.
- RFC 5424: version/PRI, NILVALUE, structured-data escaping/elements, BOM/UTF-8, missing/invalid fields.
- JSON: single/array/NDJSON streaming, types, nesting/flattening, duplicate/colliding keys, limits.
- Framing: RFC 6587 octet counting, newline, partial/multiple reads, oversized and ambiguous frames.
- Normalization: canonical mappings, collision namespaces, severity/facility, raw preservation, tenant provenance.
- Query AST: validation, canonicalization, visual serialization, backend compilation, identifier/value escaping.
- Configuration: precedence, validation, redaction, unknown keys, listener conflicts.
- Auth: Argon2 parameter upgrades, session rotation/expiry/revocation, CSRF, RBAC matrix.
- Batcher/queue: byte/event bounds, timer/size flush, retries, cancellation, shutdown accounting.

Property tests assert parse never panics, normalization stays within configured size, encode/decode round-trips, query compilation cannot turn a literal into syntax, and cursor/query fingerprints cannot be mixed.

### Fuzzing

Continuous short fuzz runs and scheduled long runs cover all parsers, TCP framing, NDJSON decoder, query-state codec, LogsQL/SQL quoting, and cursor decoder. Seed corpora include RFC examples, production-like deviations, invalid UTF-8, Unicode confusables, deeply nested JSON, huge length prefixes, and injection strings.

### Contract tests

Every `LogStore` adapter runs the same black-box semantic suite:

- append and retrieve canonical/dynamic fields;
- half-open time boundary;
- text and structured predicates;
- missing/null/empty behavior;
- supported typed comparisons;
- ordering/cursor continuation with ties and concurrent inserts;
- fields, values, facets, histograms, and aggregates;
- cancellation/deadlines, partial errors, and capability reporting;
- tenant isolation and native-query restrictions.

Unsupported features must advertise and return the typed error expected by the suite.

### Integration tests

Pinned VictoriaLogs and PostgreSQL containers cover UDP/TCP -> frame -> parser -> normalize -> batch -> search; HTTP JSON/NDJSON -> persistence; API -> adapter -> response; source reconciliation; login/session/RBAC; retention status; SSE tail; streaming export; backend outage/recovery; and graceful restart.

Tests compare received/accepted/stored/dropped counters so no event disappears from accounting. Network tests use allocated non-privileged ports and explicit readiness, not sleeps.

### End-to-end/UI

Playwright validates login/logout, role navigation and 403 behavior, dashboard time range, URL-shareable visual/native searches, query builder groups, facet include/exclude, virtualized table, detail/copy actions, saved search, live reconnect/gap, source create/test/disable, export, settings, keyboard navigation, and responsive layouts. Automated accessibility checks complement manual keyboard/screen-reader review.

### Security and resilience

- Injection corpus for LogsQL, future SQL, headers, source names, CSV, and filenames.
- Stored-XSS fixtures in every displayed log field.
- Rate/size/concurrency/deadline enforcement and decompression-bomb tests.
- Slowloris TCP/HTTP, slow storage, slow export/tail client, connection churn, disk full, DB/backend restart, invalid TLS, certificate reload, and clock skew.
- Secret scanning, SAST, dependency/license scan, SBOM, image CVE scan, and secure-header checks.

## Performance and load tests

`loggen` supports RFC 3164, RFC 5424, JSON, UDP/TCP/TLS/HTTP, deterministic seeds, distributions for hosts/facilities/severities/apps/messages/custom fields, batch sizing, connections, burst/steady profiles, and target rates 1K/10K/50K/100K+. It reports attempted/sent/acknowledged errors independently of server metrics.

Benchmark matrix:

- message sizes 200 B, 1 KiB, 8 KiB, and mixed;
- 10/1K/100K host or dynamic-field cardinality;
- steady, 10x burst, storage slowdown, and concurrent queries;
- hot/cold cache queries for 5m, 1h, 24h, and retention-scale ranges;
- search text, indexed stream field, high-cardinality field, facets, top-N, histogram, tail, and export.

Record exact commit/images, host/cloud hardware, kernel, filesystem, storage, network, configuration, generator topology, corpus seed, warm-up, duration, accepted/persisted/dropped/rejected counts, CPU, RSS, GC, disk/network IO, queue depth, backend metrics, and p50/p95/p99 latency. A 100K claim requires sustained success, stated loss policy, post-run persisted reconciliation, and reproducible artifacts. Profile before tuning.

## CI gates

Pull requests run formatting, lint/vet, unit/race tests, frontend type/lint/unit tests, OpenAPI lint/breaking check, fast fuzz seeds, contract tests, integration smoke, secret scan, and build. Nightly runs full integration/E2E, longer fuzz/race, vulnerability/image scans, and a small performance regression suite. Release candidates run soak/load, restore drill, upgrade/rollback, browser matrix, and security checklist.

Coverage percentage is a signal, not the goal. Critical parsers, authorization, tenant constraints, query escaping, cursors, queue accounting, and migrations require branch/negative-path coverage and review.

## Test data

Synthetic fixtures contain no customer data. Golden records are small, attributed when derived from standards, and versioned. Large generated corpora are reproducible by seed and stored as manifests/results rather than committed blobs. Test logs include sensitive-looking values to verify redaction without using real secrets.
