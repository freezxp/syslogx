# Syslogx requirements

Status: Phase 0 baseline
Last reviewed: 2026-09-14

## Purpose

Syslogx is a self-hosted syslog ingestion and log-analysis platform for NOC, SOC, and operations teams. It must reliably accept heterogeneous events, preserve their original content, normalize common attributes, and make both known and previously unseen fields searchable without requiring per-source schemas.

This document turns the product brief into testable scope. Performance numbers are design targets until a published benchmark records the hardware, data set, configuration, duration, and loss rate.

## Users and primary workflows

- **Viewer:** inspect dashboards, search logs, use live tail, and inspect an event.
- **Operator:** Viewer permissions plus export and saved-search management.
- **Admin:** Operator permissions plus users, sources, storage, retention, and system settings.
- **Log sender:** submit syslog or HTTP events with a well-defined acceptance contract.
- **Platform operator:** deploy, upgrade, back up, restore, monitor, and diagnose Syslogx.

Critical workflows are incident search, visual filtering, field discovery, live observation, source onboarding, and platform health diagnosis.

## Functional requirements

### Ingestion

| ID | Requirement | Phase | Acceptance evidence |
|---|---|---:|---|
| ING-001 | Configurable RFC 3164 and RFC 5424 listeners over UDP and TCP | 1 | Parser and end-to-end tests with non-default ports |
| ING-002 | TLS syslog listener with configurable certificate and client-auth policy | 2/7 | TLS and mTLS integration tests |
| ING-003 | HTTP endpoint accepts one JSON object, arrays, and NDJSON | 2 | Content-type and partial-error tests |
| ING-004 | Unknown fields and original raw input are preserved within documented limits | 1/2 | Round-trip tests |
| ING-005 | Parsing, normalization, buffering, and storage are independently replaceable stages | 1 | Package boundaries and contract tests |
| ING-006 | Bounded queues, batching, retry, backoff, overload policy, and graceful drain | 1 | Fault-injection and shutdown tests |
| ING-007 | Per-listener size, connection, rate, and trust limits | 1/2 | Boundary and abuse tests |
| ING-008 | Metrics distinguish received, accepted, parsed, invalid, persisted, retried, and dropped events | 1 | Prometheus metric tests |

### Search and analytics

| ID | Requirement | Phase | Acceptance evidence |
|---|---|---:|---|
| QRY-001 | Bounded time-range search, free text, structured predicates, Boolean groups, existence, comparison, prefix, and contains operators | 3 | Backend-neutral query conformance suite |
| QRY-002 | Native backend query mode is opt-in, permission-controlled, and resource-limited | 3 | Authorization and limit tests |
| QRY-003 | Search returns a stable opaque continuation cursor rather than numeric offset pagination | 3 | Concurrent-ingestion pagination tests |
| QRY-004 | Field names, values, counts/facets, and backend capability metadata are discoverable | 3 | Adapter contract tests |
| QRY-005 | Volume, severity, host, application, source IP, facility, and format aggregates use storage-native statistics | 3/4 | Query-plan/integration checks |
| QRY-006 | JSON, NDJSON, and CSV export streams with cancellation and hard limits | 3 | Memory-bound and disconnect tests |
| QRY-007 | The same absolute UTC interval applies to results, facets, charts, export, and saved-search execution | 3/4 | Cross-endpoint consistency tests |
| QRY-008 | Live tail supports filtering, pause/resume, clear, and bounded client buffers | 3/4 | Slow-client and reconnect tests |

### Product and administration

| ID | Requirement | Phase | Acceptance evidence |
|---|---|---:|---|
| UI-001 | Dark-first responsive shell and pages for login, dashboard, logs, live tail, sources, searches, analytics, system, users, storage, retention, and settings | 4/5 | Component and E2E coverage |
| UI-002 | Virtualized, keyboard-accessible log table; expandable detail; configurable columns | 4 | Accessibility and large-result UI tests |
| UI-003 | URL is the canonical shareable representation of search, time range, columns, and mode | 4 | Route round-trip tests |
| UI-004 | Visual query builder round-trips a supported query AST without semantic changes | 4 | Property/round-trip tests |
| CTL-001 | CRUD and lifecycle operations for persisted source definitions | 5 | RBAC and listener reconciliation tests |
| CTL-002 | Saved searches record owner, name, description, query mode/body, time-range policy, and timestamps | 3/4 | API tests |
| CTL-003 | Retention configuration reports desired and observed backend state | 5 | Adapter and restart tests |
| AUTH-001 | Password login, server-side session, logout, CSRF defense, and secure password hashing | 2 | Security integration tests |
| AUTH-002 | Central RBAC policy supports Admin, Operator, and Viewer and named permissions | 2 | Policy matrix tests |
| AUD-001 | Authentication and all administrative changes produce tamper-evident-at-rest audit records | 2/5 | Audit completeness tests |

## Non-functional requirements

### Reliability and semantics

- UDP is explicitly best effort; the OS or application may drop datagrams under overload.
- TCP/TLS acceptance means bytes were framed and admitted to the configured ingestion policy, not that a sender received a per-message durable acknowledgment (syslog has no standard application acknowledgment).
- HTTP `202 Accepted` means the event has passed validation and reached the configured acceptance boundary. Phase 1's default boundary is the bounded in-process queue; a durable local spool is required before claiming survival across process or storage outages.
- Queue capacity is finite. Every overload mode is explicit: block (bounded by deadline), reject/close, or drop according to source configuration. No unbounded memory structure is permitted.
- Graceful shutdown stops accepting new work, drains within a deadline, flushes batches, then accounts for remaining events as dropped or spooled.

### Initial service objectives

These are objectives for a defined reference deployment, not current claims:

| Measure | Phase 1 objective | Scale objective |
|---|---:|---:|
| Sustained ingestion | 10,000 events/s for 30 min, zero app-level drops | 100,000+ events/s after Phase 6 benchmark |
| API availability | 99.9% monthly excluding storage outage | 99.95% in HA topology |
| Recent search latency | p95 < 2 s for a selective 15-minute query | Workload-specific benchmark |
| Dashboard latency | p95 < 3 s for 1-hour overview | Workload-specific benchmark |
| Ingestion overhead | p99 admission < 100 ms for HTTP under nominal load | Workload-specific benchmark |
| Recovery | RPO/RTO documented by deployment profile | HA profile target RPO < 1 min, RTO < 15 min |

Search latency cannot be guaranteed independent of time range, selectivity, data volume, cache state, hardware, and concurrent workload. The API therefore enforces deadlines and cost limits.

### Security and privacy

- Deny by default; authorization occurs at use-case boundaries, not only in HTTP handlers.
- TLS is required outside trusted development networks. Secrets never appear in configuration output or logs.
- Tenant identity is derived from trusted authentication/listener configuration, never an untrusted event field.
- Query time range, result count, response bytes, concurrency, and duration are bounded.
- Raw logs are untrusted display content and must never be interpreted as HTML.
- Passwords use Argon2id with versioned parameters; session tokens are random, hashed at rest, rotated on login, and carried in Secure/HttpOnly/SameSite cookies.

### Operability

- `/health` is process liveness; `/ready` verifies required dependencies and admission capacity; `/metrics` is Prometheus format and separately protectable.
- Structured JSON logs contain request/correlation IDs, component, outcome, latency, and safe error classification.
- Configuration precedence is CLI > environment > YAML > defaults and the effective redacted configuration is inspectable.
- Images run as non-root except where a documented host capability is needed for privileged port binding.

## Constraints and deliberate non-goals

- Phase 0 produces documents and contracts only.
- Phase 1 is a single-node development/early-production profile; no unverified HA claim.
- Full multi-tenancy, alerting, anomaly detection, AI assistance, CEF/LEEF, Windows events, and OpenTelemetry ingestion are not Phase 1 features.
- Syslogx will not invent a universal text language that masks every backend feature. The visual AST is portable; advanced mode is explicitly native.
- Exactly-once delivery is not promised. Idempotency keys and deterministic event IDs can provide deduplication where transports and backends support it.
- The browser never connects directly to the storage engine.

## Phase 1 completion interpretation

The broad product definition of done spans Phases 1–5. The narrower **Core Ingestion Phase 1 gate** is: Compose starts Syslogx and VictoriaLogs; RFC 3164/5424 UDP/TCP messages are normalized and stored; a minimal authenticated search surface can demonstrate retrieval; health and core ingestion metrics are observable. The complete 14-item product acceptance list becomes the **MVP release gate** after Phase 5. This removes a conflict between the requested phase breakdown and its final definition of done.

## Traceability

Every roadmap milestone references requirement IDs. Every implementation PR must add or update tests, docs, metrics, and threat considerations appropriate to its change. Unsupported backend capabilities must return a typed `capability_not_supported` response; silent semantic degradation is forbidden.
