# ADR 0003: Portable query AST with explicit native dialects

- Status: Accepted
- Date: 2026-09-14

## Context

A lowest-common-denominator string language would either leak LogsQL everywhere or obscure useful backend features. Dynamically concatenating user input also creates injection risk.

## Decision

Represent visual queries as a versioned typed expression tree. Adapters compile it with grammar-aware escaping and publish capabilities. Native queries are a separate `{dialect,text}` variant, permission-gated, bounded, audited, and labeled non-portable.

## Consequences

Visual saved searches can move between conforming backends. Unsupported semantics fail explicitly. Native users keep full LogsQL/SQL power but those searches do not automatically migrate. Query compiler property and conformance tests are security-critical.
