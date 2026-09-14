# Kubernetes deployment

Production Kubernetes manifests and Helm packaging are intentionally deferred until the Compose profile and Phase 6 capacity tests establish resource, storage, rollout, and scaling requirements.

The application is Kubernetes-ready in these respects: configuration is external, the process is non-root/container-friendly, health and readiness are distinct, metrics use Prometheus exposition, shutdown drains within a deadline, state lives in external stores, and HTTP/syslog ports are separate. Future manifests must add tested security contexts, network policies, persistent storage policies, disruption budgets, topology spread, source-listener ownership, backup/restore jobs, and an explicit VictoriaLogs HA topology.

Do not treat this placeholder as a supported deployment artifact.
