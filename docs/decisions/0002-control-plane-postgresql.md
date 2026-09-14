# ADR 0002: PostgreSQL owns control-plane state

- Status: Proposed
- Date: 2026-09-14

## Context

Users, sessions, roles, source definitions, saved searches, settings, and audit records require transactions, constraints, migrations, and predictable point lookups. A log engine is not an appropriate system of record for them.

## Decision

Use PostgreSQL for production control-plane state. Keep repositories behind focused ports. SQLite may later be an explicit local developer profile only.

## Consequences

Deployment gains a component but avoids abusing the log store and supports stateless API replicas. Compose includes PostgreSQL. Availability behavior distinguishes control-plane dependency failure from continued ingestion using the last valid runtime source snapshot.
