# Architecture decision records

ADRs capture decisions that are expensive or risky to change. Records were accepted when Phase 1 was authorized on 2026-09-14. Superseded decisions remain and link to their replacement.

| ADR | Decision | Status |
|---|---|---|
| [0001](0001-victorialogs-first.md) | VictoriaLogs first | Accepted |
| [0002](0002-control-plane-postgresql.md) | PostgreSQL control plane | Accepted |
| [0003](0003-portable-query-ast.md) | Portable query AST and native dialects | Accepted |
| [0004](0004-modular-monolith.md) | Modular monolith first | Accepted |
| [0005](0005-delivery-semantics.md) | Explicit bounded delivery semantics | Accepted |
| [0006](0006-sse-live-tail.md) | SSE for browser live tail | Accepted |

New ADRs use the next four-digit number. Material changes to storage semantics, data ownership, tenancy, authentication, delivery guarantees, or deployment topology require an ADR.
