# Phase 1 configuration reference

Configuration precedence is CLI flags over environment variables over YAML over built-in defaults. YAML decoding is strict: unknown keys fail startup. Durations use Go duration syntax such as `25ms`, `10s`, or `5m`.

## Server

`server.http.address` sets the HTTP bind address. Read, write, idle, and header timeouts must be positive. CLI `-http-address` and `SYSLOGX_HTTP_ADDRESS` override the YAML address.

## Ingestion queue and batch

- `ingestion.queue.max_events` and `max_bytes` jointly bound queued frames.
- `enqueue_timeout` is the maximum admission wait before a message is accounted as dropped.
- `ingestion.batch.max_events`, `max_bytes`, and `max_age` select the first flush condition. Batch bounds cannot exceed queue bounds.
- Retry attempts use bounded exponential backoff between `initial_backoff` and `max_backoff`.

These values affect memory, latency, throughput, and loss behavior. Change them only with queue and storage metrics visible.

## Sources

Each `ingestion.sources` entry has a unique `id`, display `name`, `protocol` (`udp` or `tcp`), bind `address`, `enabled`, parser (`auto`, `rfc3164`, `rfc5424`), tenant, timezone, and maximum message size. TCP adds `max_connections` and framing (`auto`, `newline`, `octet_counting`). Ports are never hardcoded.

RFC 3164 lacks year and timezone; `timezone` supplies the listener assumption and the nearest plausible year is selected relative to receive time. UTC is the safe default. Auto framing recognizes RFC 6587 octet-counting when a decimal length prefix and space are present; otherwise it uses newline framing.

## Storage

Phase 1 accepts `storage.type: victorialogs`. `endpoint` must be an absolute HTTP(S) URL and can be overridden by CLI `-storage-endpoint` or `SYSLOGX_STORAGE_ENDPOINT`. `stream_fields` should contain only stable, bounded-cardinality identity fields. Never add event IDs, trace IDs, usernames, arbitrary IPs, or other unbounded values without a cardinality benchmark.

## Control store

PostgreSQL is enabled with `control_store.enabled`. Supply one of `dsn` or `dsn_file`; `dsn_file` is preferred for secrets. Environment overrides are `SYSLOGX_CONTROL_STORE_ENABLED`, `SYSLOGX_CONTROL_STORE_DSN`, and `SYSLOGX_CONTROL_STORE_DSN_FILE`. DSNs are never logged.

## Shutdown

`shutdown.timeout` bounds listener stop, queue drain, batch flush, and HTTP shutdown. Remaining events are accounted as dropped if the deadline is exhausted; Phase 1 has no durable spool.
