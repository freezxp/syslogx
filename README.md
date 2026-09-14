# Syslogx

Syslogx is a production-oriented syslog ingestion and log analytics platform. Phase 1 provides configurable RFC 3164/RFC 5424 ingestion over UDP and TCP, bounded batching and retry, VictoriaLogs storage, PostgreSQL control-plane foundations, health/readiness checks, and Prometheus metrics.

A dark-first React operations console provides a dashboard, bounded recent-log explorer, dynamic event details, source overview, and system readiness. HTTP JSON ingestion, authentication, the full historical query API, exports, saved searches, and live SSE tail remain planned phases. See [the roadmap](docs/roadmap.md).

## Quick start

Requirements: Docker with Compose and free host ports 514 TCP/UDP and 8080 TCP.

```bash
docker compose up -d --build
docker compose ps
curl http://127.0.0.1:8080/ready
```

Open the web console at [http://127.0.0.1:8080](http://127.0.0.1:8080).

Send a message:

```bash
logger --server 127.0.0.1 --udp --port 514 "Test syslog message"
```

Allow up to the configured one-second batch interval, then verify through Syslogx's Phase 1 diagnostic endpoint:

```bash
curl 'http://127.0.0.1:8080/api/v1/system/logs/recent?limit=20'
```

Prometheus metrics are at `http://127.0.0.1:8080/metrics`; process liveness and dependency readiness are `/health` and `/ready`.

## Configuration

The default Compose configuration is [`config/syslogx.yaml`](config/syslogx.yaml). Listener ports are not hardcoded: edit the source `address` values and Compose mappings as needed. Supported environment overrides in Phase 1 are:

- `SYSLOGX_HTTP_ADDRESS`
- `SYSLOGX_STORAGE_ENDPOINT`
- `SYSLOGX_CONTROL_STORE_ENABLED`
- `SYSLOGX_CONTROL_STORE_DSN`
- `SYSLOGX_CONTROL_STORE_DSN_FILE`

CLI flags override environment and YAML for the HTTP address and storage endpoint. Phase 1 acceptance uses an in-memory bounded queue and does not claim crash-durable delivery; UDP is best effort.

## Development

Use Go 1.27 or newer within the supported Go 1.27 line:

```bash
go test ./...
go test -race ./...
go vet ./...
```

Architecture and operational constraints are documented under [`docs/`](docs/architecture.md).
