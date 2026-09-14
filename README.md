# Syslogx

Syslogx is a production-oriented syslog ingestion and log analytics platform. Phases 1–4 provide configurable RFC 3164/RFC 5424 ingestion over UDP and TCP, JSON/NDJSON HTTP ingestion, bounded batching, VictoriaLogs storage, authenticated API foundations, search and analytics APIs, exports, saved searches, and a full log exploration UI.

A dark-first React console provides dashboards, visual and native queries, custom time ranges, dynamic fields, a virtualized result table, detail inspection, saved searches, exports, and bounded live tailing over SSE. Source administration and persisted user management remain Phase 5 work. See [the roadmap](docs/roadmap.md).

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

Allow up to the configured one-second batch interval, then open Log Explorer or query the API.

```bash
curl 'http://127.0.0.1:8080/api/v1/system/logs/recent?limit=20'
```

HTTP ingestion accepts one object, an array, or NDJSON (up to 1,000 records and 10 MiB per request):

```bash
curl -X POST http://127.0.0.1:8080/api/v1/ingest \
  -H 'Content-Type: application/json' \
  -d '{"timestamp":"2026-09-14T14:30:00Z","hostname":"server01","level":"error","service":"nginx","message":"connection refused","request_id":"demo-1"}'
```

Prometheus metrics are at `http://127.0.0.1:8080/metrics`; process liveness and dependency readiness are `/health` and `/ready`.

## Configuration

The default Compose configuration is [`config/syslogx.yaml`](config/syslogx.yaml). Listener ports are not hardcoded: edit the source `address` values and Compose mappings as needed. Supported environment overrides in Phase 1 are:

- `SYSLOGX_HTTP_ADDRESS`
- `SYSLOGX_STORAGE_ENDPOINT`
- `SYSLOGX_CONTROL_STORE_ENABLED`
- `SYSLOGX_CONTROL_STORE_DSN`
- `SYSLOGX_CONTROL_STORE_DSN_FILE`

Set `SYSLOGX_ADMIN_PASSWORD` to enable cookie-based authentication with the bootstrap `admin` account. When unset, local development runs in explicit authentication-bypass mode. CLI flags override environment and YAML for the HTTP address and storage endpoint. Ingestion uses an in-memory bounded queue and does not claim crash-durable delivery; UDP is best effort.

## Development

Use Go 1.27 or newer within the supported Go 1.27 line:

```bash
go test ./...
go test -race ./...
go vet ./...
```

Architecture and operational constraints are documented under [`docs/`](docs/architecture.md).
