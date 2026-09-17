# Phase 1 installation

## Docker Compose

Install Docker Engine with the Compose plugin. Ensure host TCP/UDP port 514 and TCP port 8080 are available, then run:

```bash
docker compose up -d --build
docker compose ps
curl http://192.168.0.55:8080/ready
```

The Compose profile binds the API to loopback and maps host syslog 514 to the unprivileged container port 1514. It creates persistent volumes for VictoriaLogs and PostgreSQL. The checked-in PostgreSQL password is a development-only Compose credential; replace it through environment/secret management before any shared deployment.

Send and verify a test event as described in the [README](../README.md). `docker compose down` stops services without deleting volumes. Do not use `down -v` unless permanent data removal is intended.

## Local backend development

Use Go 1.27.0 or a compatible newer patch in the 1.27 release line:

```bash
go mod download
go test ./...
go run ./backend/cmd/syslogx -config config/syslogx.yaml
```

The supplied YAML addresses Compose service names. For a local process, set `SYSLOGX_STORAGE_ENDPOINT=http://127.0.0.1:9428`, provide a reachable PostgreSQL DSN, or use a local override file with the control store disabled for parser/listener development.

## Readiness and startup failures

`/health` proves only that the process serves HTTP. `/ready` checks ingestion admission, VictoriaLogs, and PostgreSQL when enabled. The service intentionally reports not-ready while a required dependency is unavailable. Invalid/unknown configuration keys, duplicate source IDs, invalid durations, or listener bind conflicts fail startup.

Phase 1 uses bounded memory queues but no disk spool. UDP is best effort, and process/storage outages can lose queued events. This profile is suitable for development and controlled evaluation until the reliability requirements for a production deployment are selected and tested.
