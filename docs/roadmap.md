# Implementation roadmap

Status: Phase 1 implementation active; Phase 0 review approved 2026-09-14

## Delivery rules

Each milestone is a small, reviewable vertical slice with meaningful commits. A feature is done only with tests, metrics, configuration validation, security handling, operator/user documentation, and updated OpenAPI where applicable. Performance statements require reproducible evidence. No Phase 1 application code begins until the Phase 0 review gate is accepted.

## Phase 0 — Architecture and contracts

Deliverables in this change:

- requirements and traceability baseline;
- architecture and storage comparison;
- normalized model and ingestion pipeline;
- API, frontend, security, deployment, and test designs;
- ADRs for storage, control store, query boundary, modular deployment, delivery semantics, and live transport;
- phased roadmap.

Review gate:

1. Confirm VictoriaLogs-first and PostgreSQL control plane.
2. Confirm the narrower Core Ingestion Phase 1 gate versus the MVP gate after Phase 5.
3. Confirm Phase 1 acceptance is not crash-durable and durable spool timing.
4. Confirm native LogsQL permission model and SSE tail.
5. Approve canonical field names and tenant strategy.

After approval, add `README.md`, engineering setup, exact dependency versions, `docs/openapi.yaml`, configuration reference, and initial threat-model checklist as the first implementation scaffolding commit.

## Phase 1 — Core ingestion foundation

### 1.1 Repository and lifecycle skeleton

- Go module using the then-current supported stable Go release; strict CI, lint, race test, reproducible build.
- Composition root, config load/precedence/redaction, structured logging, signal handling, `/health`, `/ready`, `/metrics`.
- PostgreSQL migrations/repository skeleton and VictoriaLogs client health/capabilities.
- Compose profile with pinned VictoriaLogs/PostgreSQL images and non-root runtime.

Requirements: ING-005, reliability/operability baseline.
Commits: `chore: initialize backend toolchain`, `feat: add configuration and service lifecycle`, `ops: add compose development stack`.

### 1.2 Domain model and parser core

- Canonical `LogEntry`, limit policy, parser registry, normalization warnings/errors.
- RFC 3164 and RFC 5424 parsers with golden/property/fuzz tests.
- VictoriaLogs mapping contract fixtures.

Requirements: ING-001, ING-004, ING-005.
Commits: `feat: add normalized log model`, `feat: add RFC3164 parser`, `feat: add RFC5424 parser`.

### 1.3 Reliable bounded pipeline

- Byte/event-bounded queue, worker pool, batch by count/bytes/age, typed retry/backoff, cancellation, drain.
- Counters/histograms and failure accounting; fault tests.
- VictoriaLogs appender using streaming/batched HTTP with deadlines.

Requirements: ING-006, ING-008.
Commits: `feat: add bounded ingestion pipeline`, `feat: add VictoriaLogs storage adapter`, `test: add storage outage and shutdown coverage`.

### 1.4 Syslog listeners and core acceptance

- UDP and TCP listeners, RFC 6587 framing, configurable source bootstrap.
- End-to-end search verification through a minimal internal/admin endpoint or storage contract—not the full UI claim.
- Compose smoke script sends `logger` event and verifies it in VictoriaLogs through Syslogx contract.

Requirements: ING-001, ING-007.
Gate: 10K events/s reference run, no application-level drops under nominal profile, all counts reconciled. No 100K claim.

## Phase 2 — HTTP JSON and identity

### 2.1 Authentication/control plane

- Users, Argon2id passwords, sessions, CSRF, RBAC policy, bootstrap admin, audit events.
- OpenAPI-generated transport types and login/logout/me endpoints.

Requirements: AUTH-001, AUTH-002, AUD-001.

### 2.2 HTTP ingestion

- Scoped hashed ingestion keys; object/array/streaming NDJSON; limits, partial errors, idempotency contract.
- JSON parser/flattening/type preservation and collision policy.
- TLS syslog with certificate reload and optional client authentication may land here if required for first production pilot.

Requirements: ING-002, ING-003, ING-004, ING-007.

### 2.3 Durable acceptance decision

Benchmark and threat/reliability review determines whether a local disk spool is mandatory before pilot. If built: checksummed segments, quotas, fsync policy, replay, cleanup, corruption tests, and recovery metrics. Do not change `202` semantics silently.

## Phase 3 — Query and analytics API

### 3.1 Portable query service

- Versioned AST, validation/canonicalization, VictoriaLogs safe compiler, absolute time constraints, selected fields.
- Signed opaque cursor and documented best-effort consistency.
- Search API and adapter conformance tests.

Requirements: QRY-001, QRY-003, QRY-007.

### 3.2 Discovery and statistics

- Field names, values/counts, facets, histograms, grouped stats, capability endpoint.
- Dashboard overview data contract; cache only measured safe aggregates.

Requirements: QRY-004, QRY-005.

### 3.3 Streaming paths

- SSE tail proxy with client quotas/gap semantics.
- JSON/NDJSON/CSV exports with cancellation, row/byte/time quotas, CSV safety, and audit.
- Permission-controlled native LogsQL mode with backend limits.

Requirements: QRY-002, QRY-006, QRY-008.

## Phase 4 — Web experience

### 4.1 Design system and application shell

- React/Vite strict TypeScript, tokenized dark theme, same-origin auth, generated API client, route permissions.
- Responsive sidebar, error/loading/partial states, accessibility baseline.

### 4.2 Log explorer

- URL state codec, time picker, visual AST builder, native editor, synchronized volume chart/facets.
- Virtualized table, columns, detail drawer, copy/filter actions, keyboard shortcuts.

Requirements: UI-001–UI-004, QRY-001, QRY-004, QRY-007.

### 4.3 Dashboard, live, saved searches

- Overview cards/charts, bounded live buffer/reconnect UX, saved-search CRUD/ownership.
- Playwright critical flows, visual and accessibility checks.

Requirements: QRY-005, QRY-008, CTL-002.

## Phase 5 — Operations and MVP gate

- Persisted source CRUD and safe supervisor reconciliation; desired/observed status and test action.
- Users/roles, storage health/capabilities, retention desired/observed, ingestion/parser dashboards, audit viewer.
- Installation, configuration, ingestion, syslog, JSON, querying, dashboards, API, security, performance, troubleshooting, and development guides.
- Backup automation/runbook plus verified restore and upgrade/rollback drill.
- Complete the 14-item product definition-of-done acceptance run from the brief.

Requirements: CTL-001, CTL-003, UI-001, operational/security requirements.

## Phase 6 — Performance and scale proof

- `loggen` deterministic UDP/TCP/TLS/HTTP generator and benchmark manifests.
- 1K/10K/50K/100K matrix, bursts, high cardinality, mixed queries, cold/warm cache, 24-hour soak.
- Profile CPU/allocation/lock/network/storage behavior; tune only observed bottlenecks.
- Evaluate separating ingestion/query deployables and adding durable collector/broker tier.
- Publish results including unsuccessful runs and exact loss semantics. Claim 100K+ only after reconciliation proves it.

## Phase 7 — Advanced capabilities

Prioritized by evidence: ClickHouse adapter and dual-write migration tools; CEF/LEEF; OTLP/OpenTelemetry mapping; Kubernetes/Helm production profile; full tenant administration/quotas; HA clustering; alerting; trace correlation; AI query/analysis service. AI receives authorized, bounded query tools and provenance—it never receives direct unrestricted storage credentials.

## Cross-phase technical debt controls

- Review dependency direction and public interfaces every milestone.
- Track capability gaps rather than backend conditionals outside adapters.
- Maintain an ADR for changes to delivery guarantees, tenant mapping, query semantics, or persisted schema.
- Keep performance baselines and cardinality budgets in CI/nightly artifacts.
- Prefer several coherent commits per phase; never land a whole phase as one commit.

## Immediate next step after review

Record requested amendments in these documents/ADRs, mark accepted ADRs, then implement only Phase 1.1. Stop for review after the Compose health stack and lifecycle skeleton before adding parsers.
