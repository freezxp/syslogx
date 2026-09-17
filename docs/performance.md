# Performance and synthetic log generation

Syslogx has not been validated at 10K, 50K, or 100K logs/s. Do not treat a generator's sent count as stored throughput. UDP sends are best effort, and the current ingestion queue is memory-bounded rather than crash-durable.

## Generator

Run from the repository root with Go 1.27 or newer:

```bash
go run ./backend/cmd/loggen --target 127.0.0.1 --protocol udp --port 514 --rate 1000 --count 10000 --format rfc5424 --seed 42
go run ./backend/cmd/loggen --target 127.0.0.1 --protocol tcp --port 514 --rate 1000 --count 10000 --format rfc3164 --seed 42
go run ./backend/cmd/loggen --target 192.168.0.55 --protocol http --port 8080 --rate 1000 --count 10000 --format json --seed 42
```

For HTTP, the generator posts JSON arrays of at most 1,000 events to `/api/v1/ingest`. For TCP it writes newline-delimited syslog; for UDP it sends one datagram per event. Formats and transports are intentionally constrained to supported combinations. `--rate` is a target, not a guarantee. `--count` bounds the run. `--seed` makes host, application, facility, severity, and synthetic IP selection reproducible; timestamps remain real-time.

The Compose frontend currently binds to `192.168.0.55:8080`. Adjust the target if the host address changes. Do not send high-rate tests to the shared/live instance without an agreed maintenance window and retention capacity.

## Benchmark method

1. Start from a known backend state or use an isolated benchmark deployment.
2. Record exact image digests, CPU/RAM/storage limits, listener and batch configuration, VictoriaLogs settings, and generator host/network placement.
3. Run 1K, 10K, 50K, and 100K targets for a sustained interval, separately for UDP, TCP, and HTTP JSON. Include burst and high-cardinality cases.
4. Record sent/accepted/stored/dropped counters from `/metrics`, VictoriaLogs ingestion counters, CPU, RAM, queue occupancy, storage latency, and p50/p95/p99 query latency.
5. Query the exact test time range after the pipeline drains. Reconcile application and backend counts; distinguish transport loss, parser rejection, queue overflow, and storage failure.
6. Repeat runs, report variance and failed runs, then perform a longer soak. Tune only measured bottlenecks.

A 100K logs/s claim requires a reproducible report with end-to-end stored-count reconciliation and query latency under load. No such report exists yet.
