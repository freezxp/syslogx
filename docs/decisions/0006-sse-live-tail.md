# ADR 0006: Server-Sent Events for browser live tail

- Status: Accepted
- Date: 2026-09-14

## Context

Browser live tail is predominantly server-to-client, needs proxy-friendly streaming and reconnection, and must not backpressure ingestion or buffer without limit.

## Decision

Proxy the backend tail through authenticated SSE. Use bounded per-client buffers, heartbeat, log/gap/warning/error events, a short replay window, and explicit matching-rate limits. Client pause disconnects/freezes rather than accumulating indefinitely.

## Consequences

SSE is simpler than WebSocket for this unidirectional flow and works with native browser reconnection. Client-to-server interactive control uses ordinary API calls/reconnects. A future bidirectional protocol can be added if measured requirements justify it.
