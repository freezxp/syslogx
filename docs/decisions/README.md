# Architecture decision records

ADRs capture decisions that are expensive or risky to change. `Proposed` records become `Accepted` only after Phase 0 review. Superseded decisions remain and link to their replacement.

| ADR | Decision | Status |
|---|---|---|
| [0001](0001-victorialogs-first.md) | VictoriaLogs first | Proposed |
| [0002](0002-control-plane-postgresql.md) | PostgreSQL control plane | Proposed |
| [0003](0003-portable-query-ast.md) | Portable query AST and native dialects | Proposed |
| [0004](0004-modular-monolith.md) | Modular monolith first | Proposed |
| [0005](0005-delivery-semantics.md) | Explicit bounded delivery semantics | Proposed |
| [0006](0006-sse-live-tail.md) | SSE for browser live tail | Proposed |

New ADRs use the next four-digit number. Material changes to storage semantics, data ownership, tenancy, authentication, delivery guarantees, or deployment topology require an ADR.
