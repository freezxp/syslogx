# ADR 0004: Begin as a modular monolith

- Status: Accepted
- Date: 2026-09-14

## Context

The system must eventually scale ingestion and queries independently, but Phase 1 has no measured bottleneck and a distributed service topology would multiply deployment and failure modes.

## Decision

Ship one Go process with strict domain/application/adapter module boundaries and lifecycle supervision. Keep ports suitable for later process extraction. PostgreSQL and VictoriaLogs remain separate services.

## Consequences

Development, debugging, Compose, and atomic configuration are simpler. A process failure affects both ingestion and API in the initial profile. Metrics split workload attribution, and Phase 6 measurements decide whether to extract ingest/query workers.
