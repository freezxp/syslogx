# Deployment architecture

Status: Phase 0 design

## Deployment profiles

### Compose development/evaluation

`docker compose up -d` starts:

- `syslogx`: Go API plus ingestion workers; serves built SPA or proxies a development frontend profile.
- `victorialogs`: single-node persistent log store.
- `postgres`: persistent control-plane store.
- optional `prometheus`: profile-gated local monitoring.

Ports are configurable. Suggested defaults map host UDP/TCP 514 to an unprivileged container port, TLS 6514 when enabled, and HTTPS/API through 8080 for development. Data volumes are named and health checks gate dependency startup. Compose secrets or mounted files hold credentials.

Single-node Compose is not HA. Its backup consists of coordinated PostgreSQL backup plus VictoriaLogs partition snapshots copied off-host; both require documented restore tests.

### Production single-site

Place an L7 reverse proxy/load balancer before API nodes and L4 endpoints before TCP/TLS ingestion. UDP should normally land on local collectors/agents or an L4 device with understood hashing/loss behavior. Run PostgreSQL with managed/replicated backup practices and VictoriaLogs on dedicated persistent storage. Separate ingest and query components only after profiling.

### Scale/HA

```text
L4 syslog / L7 HTTPS
       |
 ingest replicas       API/query replicas
       |                       |
       +------ VictoriaLogs cluster -----+
                    |
          PostgreSQL HA control plane
```

VictoriaLogs cluster requires explicit HA/replication design; sharding storage nodes alone does not replicate data. Run multiple `vlinsert`/`vlselect` endpoints and the vendor-supported HA storage topology, plus off-site backups. Source definitions use lease-based ownership for singleton binds. Stateless API replicas share session/control state in PostgreSQL.

## Image and runtime requirements

- Multi-stage reproducible builds; minimal pinned runtime base or distroless where operationally practical.
- Non-root UID/GID, read-only filesystem, no-new-privileges, dropped capabilities, explicit temp and data mounts.
- Graceful termination budget must exceed ingestion drain and load-balancer deregistration periods.
- CPU/memory requests and limits are workload-tested; avoid a tight memory limit that turns storage backpressure into OOM kills.
- Image provenance, SBOM, vulnerability scan, and signed release artifacts are release gates.

## Configuration

Precedence: CLI flags, environment variables, YAML, defaults. Configuration is schema-validated at startup. Live-reload scope is explicit: safe listener/certificate/rate changes reconcile; storage identity and database DSN require restart initially. Unknown keys and conflicting listener binds are errors.

Example shape (not yet an implementation contract):

```yaml
server:
  http:
    address: ":8080"
  admin:
    address: ":9090"
ingestion:
  queue:
    max_events: 100000
    max_bytes: 268435456
  sources_file: /etc/syslogx/sources.yaml
storage:
  type: victorialogs
  endpoint: http://victorialogs:9428
  request_timeout: 10s
control_store:
  driver: postgres
  dsn_file: /run/secrets/postgres_dsn
retention:
  days: 30
```

Defaults do not enable public unauthenticated ingestion in a production profile. Secret values are never emitted by config diagnostics.

## Persistence, backup, and restore

VictoriaLogs uses a dedicated volume and configured retention. Backups create vendor-supported consistent partition snapshots, copy them to versioned off-host/object storage, verify checksums, and expire snapshots only after success. PostgreSQL uses consistent logical/physical backups with WAL strategy appropriate to RPO. The runbook defines ordering, compatible software versions, DNS/endpoint cutover, and validation queries.

A backup is not accepted until an automated or scheduled restore drill proves it. Dashboard status separates last snapshot, last off-host copy, last verification, and last restore test.

## Health and rollout

- Liveness: process event loop/runtime is functioning; no dependency checks.
- Readiness: migrations complete, control DB reachable, log backend usable, and ingestion not beyond sustained saturation threshold.
- Startup: configuration and migrations valid; separate from liveness.
- Graceful shutdown marks not-ready before closing admission and draining.

Database migrations are backward-compatible for rolling deployment (expand, migrate, contract). Adapters expose backend version/capabilities and fail readiness on unsupported versions. Rollback never assumes a destructive schema downgrade.

## Kubernetes preparation

Reserve `deploy/kubernetes/` with a README in a later implementation commit; do not ship untested manifests as production support. Design assumptions:

- Deployments for stateless API/ingest coordinators; StatefulSets/vendor chart for stateful stores.
- Services separate HTTP, TCP, UDP, TLS, and admin metrics.
- ConfigMaps for non-secrets; Secrets/external secret provider for credentials.
- PodDisruptionBudgets, topology spread, anti-affinity, network policies, service accounts, and security contexts.
- Persistent-volume performance and reclaim policy are explicit.
- Horizontal scaling uses application metrics and connection/source ownership constraints, not CPU alone.

## Upgrade and compatibility

Publish a support matrix for Syslogx, VictoriaLogs, PostgreSQL, browser, and configuration/schema versions. Pin container digests for releases. Upgrade in staging with ingestion replay/canary search comparisons, then rolling or blue/green rollout. Retention and storage-format changes require backup verification. Every release includes rollback constraints and migration notes.

## Capacity planning

Size from measured average/peak events per second, bytes per event after normalization, peak burst duration, retention, compression from representative data, query concurrency, tail/export load, replication, headroom, and backup workspace. Keep at least vendor-recommended disk/CPU headroom and alert on queue saturation, disk forecast, parts/merges, stream cardinality, rejected rows, and query saturation.
