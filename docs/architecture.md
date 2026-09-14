# Architecture

Status: proposed for Phase 0 review  
Decision baseline: VictoriaLogs first, PostgreSQL control plane, modular monolith

## Executive decision

Start with a single Go deployable containing HTTP API, source supervisors, parsers, normalization, query orchestration, and metrics. It writes log batches through a `LogStore` port to VictoriaLogs. A React single-page application speaks only to the versioned Go API. PostgreSQL stores control-plane state. Components remain internal modules with explicit ports so ingestion workers and API/query workers can become separate deployables when measurements justify it.

VictoriaLogs is the initial log backend because Phase 1 prioritizes turnkey log search, arbitrary fields, field/facet discovery, hits/statistics, streaming results, live tail, compact deployment, and LogsQL. ClickHouse remains a planned adapter when advanced SQL analytics, materialized views, richer typed schemas, or mature clustered operational tooling dominate.

## Context

```text
 Syslog devices       HTTP/agents          Browser/admin
 UDP | TCP | TLS      JSON | NDJSON        HTTPS/SSE
       |                    |                  |
       +----------+---------+                  |
                  v                            v
        +------------------ Syslogx -------------------+
        | listener -> frame -> parse -> normalize     |
        |              -> bounded buffer -> batch     |
        | API -> auth/RBAC -> application services    |
        | query AST -> planner -> LogStore adapter    |
        | source supervisor | audit | metrics         |
        +------------+--------------------+------------+
                     |                    |
                     v                    v
              VictoriaLogs          PostgreSQL
              log data plane        control plane
```

Prometheus scrapes Syslogx and VictoriaLogs. An optional reverse proxy terminates public HTTPS; syslog TLS terminates in the ingestion listener. The browser has no storage credentials.

## Architectural principles

1. **One semantic core, several transports.** Network framing is not parsing; parsing is not normalization; normalization is not persistence.
2. **Bound every resource.** Bytes, frames, queue depth, batch size, retries, requests, queries, output, tail clients, and shutdown all have limits.
3. **Portable common path, explicit extensions.** A typed query AST covers product filters. Native queries are backend-qualified and never masquerade as portable.
4. **Control and data planes differ.** Relational entities and transactions belong in PostgreSQL; high-volume append/search data belongs in a log store.
5. **Tenant context is mandatory internally.** Single-tenant Phase 1 uses a default tenant, but no storage or authorization call lacks tenant scope.
6. **Measure before splitting.** A modular monolith reduces operational cost; stable ports provide an extraction path.

## Backend module boundaries

```text
backend/
  cmd/syslogx/          composition root and lifecycle
  cmd/loggen/           synthetic generator (later phase)
  internal/
    domain/             LogEntry, tenant, query AST, errors
    ingestion/          pipeline, batcher, backpressure, source runtime
    transport/          udp, tcp, tls, http framing/admission
    parser/             rfc3164, rfc5424, json registry
    normalization/      canonical mapping and limits
    storage/            ports, capability model, adapter registry
      victorialogs/     HTTP translation only
      clickhouse/       future
    query/              validation, planning, cursor signing
    api/                REST/SSE handlers and middleware
    auth/               credentials, sessions, policy
    controlstore/       PostgreSQL repositories/migrations
    metrics/            Prometheus instruments
    config/             load, merge, validate, redact
    audit/              security and administrative events
  pkg/                  only intentionally public reusable packages
  tests/                integration fixtures and black-box suites
```

Dependencies point inward: adapters depend on application/domain ports. Domain packages import neither VictoriaLogs nor HTTP framework types. Cross-module behavior is invoked through small interfaces; avoid a generic repository abstraction that erases useful storage semantics.

## Runtime components

### Source supervisor

Reconciles desired listener definitions with running listeners. A listener has an immutable ID, protocol, bind address, parser policy, limits, TLS secret reference, enabled state, and revision. Changes are applied as start-new/health-check/stop-old where the OS permits; bind conflicts fail without destroying the working listener. Static bootstrap YAML remains available if PostgreSQL is unavailable.

### Ingestion pipeline

Each accepted frame becomes an envelope carrying immutable transport metadata and ownership. Parser selection yields a parsed event or a classified failure. Normalization maps canonical fields, caps field count/value size, and preserves raw input. A bounded multi-producer queue feeds size/time-bounded batches to the adapter. Retryable errors use exponential backoff with jitter and a retry budget; permanent errors are counted and sampled safely.

### Query service

The service validates an absolute half-open UTC interval `[start,end)`, tenant, permissions, requested fields, AST complexity, limit, deadline, and backend capabilities. The adapter compiles the AST and applies mandatory tenant/time filters separately from user input. Results stream where practical. Cursors are opaque, signed, versioned envelopes tied to the query fingerprint and direction.

### Tail broker

The backend consumes VictoriaLogs live tail and exposes SSE to browsers. SSE is preferred for a unidirectional stream, proxy compatibility, and native reconnection. Each client has a bounded ring buffer; slow clients receive a gap marker or are disconnected. Pause is a browser state, not infinite server buffering. Tail is for human-scale matching rates, not bulk export.

### Control store

PostgreSQL repositories own users, roles, sessions, sources, saved searches, settings, audit records, and schema migrations. SQLite may be supported only as an explicit developer profile, not as the default production topology. Storage credentials and TLS keys are references to mounted/environment secrets, never database values by default.

## Storage port design

Avoid one oversized interface. Use capability-oriented ports:

```go
type Appender interface {
    Append(context.Context, []LogEntry) (AppendResult, error)
}

type Searcher interface {
    Search(context.Context, SearchRequest) (LogStream, error)
}

type Aggregator interface {
    Aggregate(context.Context, AggregateRequest) (AggregateResult, error)
}

type FieldDiscoverer interface {
    Fields(context.Context, FieldRequest) ([]FieldInfo, error)
    Values(context.Context, FieldValueRequest) ([]FieldValue, error)
    Facets(context.Context, FacetRequest) ([]Facet, error)
}

type Tailer interface {
    Tail(context.Context, TailRequest) (LogStream, error)
}

type RetentionController interface {
    Retention(context.Context) (RetentionStatus, error)
    ConfigureRetention(context.Context, RetentionPolicy) error
}

type Backend interface {
    Appender
    Searcher
    Aggregator
    Capabilities(context.Context) Capabilities
    Health(context.Context) BackendHealth
}
```

Optional ports are obtained by type assertion/registry and reflected by `Capabilities`; this prevents fake implementations. Domain DTOs contain no LogsQL or SQL. `AdvancedQuery{Dialect, Text}` is a separate union requiring `query.native` permission. Adapter contract tests run the same semantic fixtures against every backend.

## Query representation

The visual query is a versioned expression tree:

```text
Expr = And(Expr...) | Or(Expr...) | Not(Expr)
     | Text(value)
     | Compare(field, eq|neq|gt|gte|lt|lte, typedValue)
     | Match(field, contains|prefix, value)
     | Exists(field)
```

Time, tenant, selected columns, ordering, and page limit are request properties, not textual clauses. Field names and values remain distinct typed nodes through compilation. Numeric comparison requires an explicit numeric interpretation because VictoriaLogs stores field values as strings; adapters may reject unsupported or ambiguous operations.

## Scale evolution

### Profile A: Compose / single node

One Syslogx process, one VictoriaLogs instance, PostgreSQL, and web assets. This is the Phase 1 developer and evaluation profile.

### Profile B: separated workloads

Stateless API/query replicas and ingestion replicas share PostgreSQL and use VictoriaLogs cluster endpoints. UDP requires L4 load balancing or host-local agents. Listener ownership uses leases to avoid multiple replicas binding/processing a singleton source definition.

### Profile C: durable edge buffering

For strict outage tolerance, insert a documented durable spool or collector tier between listeners and storage. Kafka is not mandatory by default; it adds meaningful cost and is introduced only when replay, multi-consumer fan-out, or outage RPO requires it. The `Appender` boundary remains unchanged.

VictoriaLogs cluster shards rather than automatically replicating logs, so HA requires the vendor's documented HA topology plus backups; merely adding storage nodes is not replication.

## Failure behavior

| Failure | Behavior |
|---|---|
| Malformed message | Classify, count, optionally store as `format=unknown`; never crash listener |
| Queue full | Apply per-source deadline/policy; account every rejection/drop |
| Storage timeout | Retry within budget; readiness becomes false at sustained saturation |
| PostgreSQL unavailable | Existing listeners may continue from last good snapshot; mutations/login requiring DB fail closed |
| Tail client slow | Bounded buffer, gap event/disconnect; no ingestion impact |
| Query canceled | Propagate context cancellation to adapter/backend immediately |
| Shutdown | Stop admission, drain to deadline, flush/account remainder |

## Deployment and observability

The Go process exposes public API and separate admin/metrics endpoints where configured. Readiness depends on control DB, log backend, migration state, and ingestion saturation; liveness never depends on an external service. Metrics use low-cardinality labels (source ID is opt-in/bounded); raw hostnames, queries, users, and error messages are not metric labels.

## Architectural risks

- VictoriaLogs uses string-valued flat fields, so typed comparison semantics need a canonical representation and capability disclosure.
- Dynamic field names can create field explosion; ingestion limits and field-name policy are mandatory.
- A single process with an in-memory queue cannot promise durable acceptance through a crash; documentation and later spool work must be explicit.
- Advanced native query access can consume backend resources; permissions and backend-enforced limits are required.
- Runtime management of privileged ports is operationally sensitive; container capabilities or high-port mapping are preferable to root.

## Primary references

- [VictoriaLogs key concepts and arbitrary fields](https://docs.victoriametrics.com/victorialogs/keyconcepts/)
- [VictoriaLogs query, facets, statistics, and tail APIs](https://docs.victoriametrics.com/victorialogs/querying/)
- [VictoriaLogs cluster architecture](https://docs.victoriametrics.com/victorialogs/cluster/)
- [ClickHouse observability architecture guidance](https://clickhouse.com/docs/guides/use-cases/observability/build-your-own/introduction)

Detailed decisions are recorded under `docs/decisions/`.
