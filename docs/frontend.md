# Frontend architecture and UX

Status: Phase 0 design

## Stack

Use TypeScript in strict mode, current stable React, Vite, Tailwind CSS, an accessible headless component system, TanStack Router (or React Router if selected by ADR), TanStack Query, TanStack Table, TanStack Virtual, and Recharts. Pin exact versions only at implementation time and record them in the lockfile; avoid planning against speculative version numbers.

The frontend is a static SPA served behind the same origin as the API. Generated OpenAPI types cover transport DTOs. Domain/view models and query AST helpers are handwritten and tested. No storage-specific query construction occurs in UI components.

## Application layers

```text
app shell/routes
  -> page features (logs, dashboard, live, sources, searches, system)
     -> domain hooks/view models/query-state codec
        -> generated API client + SSE client
           -> design system primitives
```

Feature folders own route, components, hooks, tests, and state codec. Shared code is limited to design primitives, API infrastructure, authentication, query AST, time range, and formatting. TanStack Query owns remote state; URL parameters own shareable search state; small local stores/hooks own ephemeral UI state. Avoid duplicating server data into a global store.

## Information architecture

Primary navigation:

- Dashboard
- Explore: Logs, Live Tail
- Sources
- Saved Searches
- Analytics
- System: Ingestion, Storage, Health, Metrics
- Administration: Users, Settings, Retention

Routes include `/login`, `/dashboard`, `/logs`, `/logs/live`, `/sources`, `/sources/:id`, `/searches`, `/searches/:id`, `/analytics`, `/settings`, `/settings/storage`, `/settings/retention`, `/settings/users`, and `/settings/system`. `/` redirects based on authentication and preference.

Navigation is permission-filtered for convenience, but the server remains authoritative. Deep links to unauthorized pages show a clear 403 state.

## Visual language

Dark mode is the default, with neutral charcoal surfaces, restrained borders, accessible semantic severity colors, tabular numerals for metrics/timestamps, and dense 32–36 px table rows. Animation is limited to state transitions that communicate change and respects reduced motion. Typography, spacing, color, focus rings, layer elevation, and chart palette are design tokens, not ad hoc Tailwind strings.

Target WCAG 2.2 AA for controls, text, focus, keyboard flow, and non-color severity cues. All chart insights also have tabular/text alternatives.

## Log explorer

Page regions:

1. Sticky time range, timezone, refresh, and run controls.
2. Visual/Advanced query editor with validation and keyboard execution.
3. Volume histogram synchronized to the exact query and absolute interval; brushing updates time range.
4. Collapsible facet rail with searchable field list, top values, include/exclude actions, truncation/approximation states.
5. Virtualized table with user-selected columns, resizing, pinning, stable timestamp formatting, and expandable rows.
6. Side drawer detail so table context remains visible.

The query runs only on explicit Run/keyboard submit by default; facet interactions may auto-run with cancellation/debounce. Previous results remain visible with a loading indicator to avoid disruptive blanking. Abort superseded requests.

### URL state

The URL encodes versioned visual AST (compact base64url JSON or readable repeated filter parameters), native mode/dialect/text, absolute `from`/`to`, display timezone, columns, sort, and optional saved-search ID. Relative presets resolve to absolute bounds on Run so all panels agree. Cursor and transient drawer state need not be shareable. Invalid/old state is migrated or rejected visibly, never silently reinterpreted.

### Query builder

Rows provide field, operator, typed value, and conjunction/group controls. Field choices are discovered for the active interval; arbitrary names remain possible. Operators are filtered by inferred type and backend capabilities. AST is canonical; displayed advanced syntax is a compiler output, not parsed back unless a dialect parser explicitly supports lossless conversion. Switching an unsupported native query to visual mode warns rather than discarding semantics.

Keyboard: `/` focuses query, `Ctrl/Cmd+Enter` runs, `t` opens time range outside text inputs, arrow/Enter navigation works in field/value menus, and `Esc` closes drawers. A shortcut help overlay is available.

## Log detail

The drawer shows canonical fields first, message with safe wrapping, dynamic fields sorted/searchable, labels, normalized JSON, and raw message. Each field offers include, exclude, copy, and add column based on permission/capability. Large values are initially collapsed. Copy actions use plain text; log content is never injected as HTML.

## Live tail

An SSE client feeds a bounded browser ring buffer. Controls include connect/disconnect, pause/resume, clear, auto-scroll, maximum records, local find/highlight, and server-side filter apply. Pausing cancels or visually freezes; it does not accumulate without bound. Gap/drop indicators remain visible. Auto-scroll disables when the user scrolls away and resumes only explicitly.

## Dashboard

Cards: total logs, computed ingestion rate, logs today, errors, active sources, and storage used, each with freshness/source metadata. Charts: rate/volume, severity, top hosts/apps/source IPs/facilities/formats. A shared time context makes requests concurrently and independently retryable. Metric cards derived from Prometheus are labeled separately from log-store statistics to avoid pretending different clocks are identical.

## Data handling and performance

- Search pages fetch 100–500 rows and follow cursors; virtualization controls DOM size, not server result size.
- Facets, volume, and rows use separate cached queries sharing a normalized key.
- Debounce suggestions, cancel superseded requests, and cap concurrent facet calls.
- Lazy-load route bundles and heavy chart/editor code.
- Use Web Workers only if profiling shows query serialization or large JSON formatting blocks the main thread.
- Frontend telemetry records route/API/Web Vitals and safe error codes, never log content or query text by default.

## States and failure UX

Every region defines loading, empty, stale, partial, unauthorized, rate-limited, timeout, backend unavailable, and validation states. Partial backend results are visibly marked. The UI shows request IDs for support. Destructive settings require confirmation and display desired versus observed reconciliation state.

## Testing

Vitest and Testing Library cover state codecs, query editing, permissions, detail actions, and accessibility. MSW exercises transport behavior. Playwright covers login, URL-shared search, facet filtering, cursor scrolling, saved searches, tail reconnect/gap, source changes, export, and responsive/keyboard paths. Visual regression targets a small set of dense critical screens.
