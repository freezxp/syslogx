# ADR 0005: Bounded at-least-accounted ingestion, not implied durability

- Status: Accepted
- Date: 2026-09-14

## Context

Backpressure and bounded queues protect the process, but an in-memory queue cannot survive crashes or long backend outages. UDP and standard syslog do not provide end-to-end acknowledgments.

## Decision

Document transport-specific acceptance boundaries. Bound queues by bytes and events, apply explicit overload policies, batch writes, retry within budgets, and account every terminal outcome. Do not claim durable acceptance until a tested disk spool/WAL is implemented. Expect possible duplicates on retry.

## Consequences

Phase 1 is honest and operationally safe but can lose admitted in-memory events on crash/outage and UDP under load. Strict-RPO deployments will require the planned spool/collector tier. Metrics and documentation distinguish received, accepted, stored, and dropped.
