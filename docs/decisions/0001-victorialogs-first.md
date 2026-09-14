# ADR 0001: VictoriaLogs is the initial log backend

- Status: Proposed
- Date: 2026-09-14

## Context

Syslogx needs arbitrary fields, full-text search, field/value/facet discovery, time histograms, statistics, streaming results, retention, and live tail with a small Phase 1 operational footprint.

## Decision

Implement VictoriaLogs first behind capability-oriented log storage ports. Preserve a portable visual query AST and expose LogsQL only as an explicitly native advanced dialect.

## Consequences

Phase 1 reaches product search primitives with less custom database work and a compact Compose deployment. String-valued fields constrain typed comparisons, snapshot/HA operations need careful runbooks, and native saved searches are vendor-bound. Adapter conformance tests and capabilities prevent VictoriaLogs semantics leaking into the rest of the application. ClickHouse is the planned second adapter, triggered by documented analytics/operations needs and benchmarks.
