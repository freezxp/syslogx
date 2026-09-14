# API architecture

Status: design contract; the executable OpenAPI document is a Phase 1/3 deliverable

## Conventions

- Base path `/api/v1`; JSON uses UTF-8 and `snake_case` fields.
- Timestamps are RFC 3339 with UTC `Z`; intervals are half-open `[start,end)` and both bounds are required for log queries.
- IDs are opaque strings. Durations are ISO 8601 or documented Go-style strings only where configuration uses them.
- Errors use `application/problem+json` with stable `type`, `title`, `status`, `code`, `detail`, `instance`, `request_id`, and optional field violations.
- API evolution is additive within v1. Breaking semantics require v2. Deprecations include response headers and release-note dates.
- Browser authentication uses a Secure/HttpOnly/SameSite session cookie and CSRF token for mutations. Ingestion uses scoped bearer/API credentials.
- Every endpoint has request bytes, response bytes, duration, concurrency, and rate limits appropriate to its workload.

## Resource surface

| Method/path | Purpose | Permission |
|---|---|---|
| `POST /api/v1/auth/login` | Establish session and rotate token | public/rate-limited |
| `POST /api/v1/auth/logout` | Revoke current session | authenticated |
| `GET /api/v1/auth/me` | Current user, roles, permissions | authenticated |
| `POST /api/v1/ingest` | Single, batch, or NDJSON events | `ingest.logs` credential |
| `POST /api/v1/logs/search` | Portable/native search with cursor | `logs.search` |
| `POST /api/v1/logs/stats` | Histograms and grouped aggregates | `logs.search` |
| `GET /api/v1/logs/tail` | SSE live tail | `logs.tail` |
| `POST /api/v1/logs/export` | Streaming JSON/NDJSON/CSV | `logs.export` |
| `POST /api/v1/fields` | Discover fields in query/time scope | `logs.search` |
| `POST /api/v1/fields/{field}/values` | Values and counts | `logs.search` |
| `POST /api/v1/facets` | Multi-field facets | `logs.search` |
| CRUD `/api/v1/sources` | Desired listener definitions and status | `sources.manage` or read variant |
| CRUD `/api/v1/saved-searches` | Saved portable/native searches | owner/admin policy |
| `GET /api/v1/dashboard/overview` | Summary cards | `logs.view` |
| `POST /api/v1/dashboard/volume` | Time-bucketed charts | `logs.view` |
| `/api/v1/users`, `/roles` | User and role administration | `users.manage` |
| `/api/v1/settings/*` | Storage, retention, and system desired/observed state | corresponding manage permission |
| `GET /api/v1/capabilities` | Backend/product feature flags and limits | authenticated |
| `GET /health` | Process liveness | deployment policy |
| `GET /ready` | Dependency/admission readiness | deployment policy |
| `GET /metrics` | Prometheus metrics | separate admin policy |

Use `POST` for complex searches so typed JSON is not constrained by URL length. The UI still serializes portable search state into its browser URL; it posts the decoded request to the API.

## Search request

```json
{
  "time_range": {
    "start": "2026-09-14T10:00:00Z",
    "end": "2026-09-14T12:00:00Z"
  },
  "query": {
    "mode": "visual",
    "expression": {
      "op": "and",
      "children": [
        {"op": "compare", "field": "severity_name", "operator": "eq", "value": "error"},
        {"op": "match", "field": "message", "operator": "contains", "value": "VPN"}
      ]
    }
  },
  "fields": ["timestamp", "hostname", "severity_name", "app_name", "message"],
  "sort": [{"field": "timestamp", "direction": "desc"}],
  "limit": 100,
  "cursor": null
}
```

The response contains `data`, `page.next_cursor`, `page.has_more`, `meta.request_id`, `meta.took_ms`, `meta.partial`, and warnings. Defaults and maximums are server-advertised. Native mode uses `{"mode":"native","dialect":"logsql","text":"..."}` and a stronger permission.

## Cursor semantics

Cursors are base64url-encoded, authenticated server envelopes containing version, backend, tenant, normalized-query hash, sort boundary, direction, and expiry. They are opaque to clients and never contain credentials. Changing query/time/sort invalidates the cursor. A bad or expired cursor returns `400 invalid_cursor`; an adapter unable to continue consistently returns an explicit capability error.

Event-time ordering under concurrent ingestion is not a snapshot guarantee. The API documents `best_effort` consistency for VictoriaLogs Phase 1, possible late records, and tie handling. Exports use a dedicated streaming plan and limits rather than repeatedly following UI cursors.

## Query validation and compilation

Validation caps AST depth, nodes, field length, value length, requested fields, time span by role, limit, and deadline. Field names and values remain separate tokens. The adapter quotes both according to the backend grammar and always adds trusted tenant/time restrictions outside user-controlled native text where possible. Native queries that cannot be safely constrained are disabled or routed through backend credentials/proxies with enforced tenant and resource policy.

Comparison rules are explicit:

- `eq`/`neq`, existence, prefix, contains, and Boolean composition are portable baseline operations.
- `gt/gte/lt/lte` require declared/coerced field type and adapter support.
- Null, missing, and empty string are distinct.
- Text tokenization/case behavior is returned in capabilities and documented per adapter.

## Fields and facets

All discovery requests carry the exact search query and time range. `FieldInfo` includes name, inferred types, approximate/known cardinality where available, stream/index hints, and capabilities. Value/facet results include counts and `approximate` flags. Limits and truncation are explicit. High-cardinality fields may return no facets while remaining searchable.

## Live tail over SSE

`GET /api/v1/logs/tail?state=<short-lived-signed-reference>` avoids placing sensitive large query text in logs. Event types are `log`, `gap`, `heartbeat`, `warning`, and `error`; each includes a monotonic stream sequence. `Last-Event-ID` supports only the server's bounded replay window. Reconnect outside that window emits `gap`; it never implies durable replay.

Pause/resume is client-side subscription cancellation/reconnect. Maximum display records live in the browser ring buffer. The server caps matching rate and disconnects persistently slow consumers.

## Streaming export

`POST /api/v1/logs/export` returns chunked `application/x-ndjson`, `application/json` (streamed array), or `text/csv`. Headers include safe content disposition, request ID, and optional result truncation metadata. The service streams adapter rows through an encoder with bounded buffers, propagates disconnect cancellation, and enforces rows, bytes, duration, and concurrent export quotas. Formula-injection-prone CSV cells are escaped/prefixed according to configured safe mode.

## Source operations

Source mutation uses optimistic concurrency (`revision`/`If-Match`). Responses distinguish desired state from runtime state (`starting`, `running`, `degraded`, `stopped`, `failed`) with redacted diagnostics. `test` validates configuration and bind/TLS reachability without replacing a healthy listener; destructive delete requires source stopped or `force` with explicit audit.

## OpenAPI process

`docs/openapi.yaml` will be the source-of-truth OpenAPI 3.1 contract when endpoint implementation begins. CI will lint it, detect breaking changes, generate TypeScript types/client primitives, and run handler contract tests. Handwritten domain types do not depend on generated transport types. SSE and NDJSON streaming semantics require prose extensions/examples in addition to schemas.
