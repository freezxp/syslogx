# Ingestion architecture

Status: Phase 0 design

## Pipeline

```text
socket/http
  -> admission limits
  -> transport framing + immutable metadata
  -> parser selection
  -> canonical normalization/enrichment
  -> bounded queue
  -> size/time batcher
  -> storage adapter
  -> metrics/outcome
```

Transport, format, and persistence are separate extension points. A `Frame` contains payload, receive time, source/tenant IDs, peer address, protocol, and connection/request correlation. Parsers are stateless where possible and return a parsed event plus warnings. Normalizers never perform network I/O.

## Transport behavior

### UDP

- One datagram is one candidate event; apply maximum datagram/message size.
- Best effort: kernel receive-buffer and application-queue loss are possible and observable where the OS exposes counters.
- Use multiple read workers only after profiling ordering and socket behavior. Never block indefinitely on downstream storage.
- Privileged port 514 is exposed by container port mapping or narrowly scoped bind capability; the process should otherwise be non-root.

### TCP and TLS

- Support RFC 6587 octet-counting and non-transparent delimiter framing. Framing mode is configurable (`auto`, `octet_counting`, `newline`) with conservative auto-detection.
- Bound line/frame size, idle time, connection lifetime as needed, concurrent connections, handshake time, and per-connection in-flight frames.
- TLS minimum version, cipher defaults, server certificate, CA bundle, and client-certificate mode are explicit. Certificate reload is atomic.
- Backpressure reaches TCP reads within configured deadlines; sustained overload closes/rejects connections rather than growing memory.

### HTTP JSON

- `POST /api/v1/ingest` supports `application/json` (object or array) and `application/x-ndjson`.
- Stream NDJSON decoding; JSON arrays are capped by bytes/event count. Never decode an unbounded request into memory.
- Default response is `202`; batch response reports accepted/rejected counts and bounded indexed errors. Atomic batch mode is optional and size-limited.
- Authentication uses scoped ingestion tokens/API keys, not browser sessions. Per-credential/source quotas and idempotency keys are planned.

## Parser registry

```go
type Parser interface {
    Name() string
    Detect(FrameView) Confidence
    Parse(context.Context, FrameView) (ParsedEvent, []Warning, error)
}
```

Configured parser choice wins. `auto` detection is ordered and bounded: RFC 5424, RFC 3164, JSON, then unknown. Detection never repeatedly copies the payload. Future CEF/LEEF, common web logs, Windows, Kubernetes, generic text, and OpenTelemetry enter through the same registry or a dedicated protocol receiver.

RFC 3164 parsing must handle missing year/timezone, common tag/PID variants, and relay-added content without accepting unlimited ambiguity. RFC 5424 handles NILVALUE, UTF-8 BOM, structured-data escaping, multiple elements, and optional message. Golden tests come from standards plus real-world deviations.

## Buffering, batching, and retries

- A global byte budget and per-source event budget bound memory; counting only event count is insufficient.
- Batch flush occurs at the first of max events, max encoded bytes, or max age.
- Worker count, queue bytes/events, and batch thresholds are configuration with safe validation.
- Adapter errors are typed `retryable`, `throttled`, `permanent`, or `canceled`.
- Retry uses capped exponential backoff with jitter and an elapsed-time/attempt budget. Circuit state prevents retry storms.
- Partial backend acceptance must identify outcomes or the whole batch is conservatively retried with possible duplicates.
- Phase 1 does not claim crash durability. A disk spool/WAL milestone adds checksummed segments, fsync policy, quotas, replay, corruption handling, and monitoring before `durable` acceptance is exposed.

## Overload policies

Each listener chooses from supported policies with protocol-appropriate defaults:

| Policy | UDP | TCP/TLS | HTTP |
|---|---|---|---|
| `drop_new` | Drop/count | Stop read then close at deadline | `429`/`503` |
| `block` | Very short bounded wait only | Backpressure read to deadline | Wait to request deadline |
| `spool` | After durable spool exists | After durable spool exists | `202` after durable append |

There is no `unbounded` option. Drop/error metrics identify stage and reason using bounded labels.

## Metrics

Counters: `messages_received_total`, `messages_accepted_total`, `messages_parsed_total`, `messages_parse_error_total`, `messages_stored_total`, `messages_dropped_total`, `messages_retried_total`, `bytes_received_total`, and `bytes_stored_total`.

Gauges/histograms: queue events/bytes/utilization, batch size/bytes, batch age, active connections, storage request duration, end-to-end persist duration, parser duration, and worker saturation. Rates such as logs/s and bytes/s are calculated with PromQL from counters, not maintained as racy application gauges.

Labels are controlled enums/IDs: transport, parser, outcome/reason, and optionally bounded source ID. Never label by host, IP, message, dynamic field, query, or user.

## Graceful shutdown

1. Mark readiness false and stop new HTTP/listener admission.
2. Close listener sockets and wait for framing workers.
3. Close the normalization output queue.
4. Drain and flush batches until deadline.
5. Persist spool state if enabled; otherwise count/log the bounded remainder.
6. Close backend/control-store connections and metrics server.

Shutdown tests prove no send-on-closed-channel races, stuck workers, or unaccounted admitted events.

## Future collector integration

OpenTelemetry Collector support should initially use OTLP into a dedicated receiver or a Collector-to-Syslogx HTTP mapping. The canonical model will map OTel resource/scope/log attributes without promoting untrusted high-cardinality attributes into stream identity. Collector deployment is complementary to native syslog listeners, not a reason to embed the entire Collector in Phase 1.
