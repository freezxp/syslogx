# Security architecture

Status: threat-model baseline

## Trust boundaries and assets

Untrusted zones are syslog senders, HTTP ingestion clients, browsers, log contents, query text, forwarded network identity, and imported configuration. Trusted-but-fallible dependencies are PostgreSQL and the log backend. High-value assets are logs, raw messages, credentials, sessions, signing/encryption keys, TLS private keys, source configuration, saved searches, audit history, and availability.

Primary threats include credential theft, tenant escape, query injection, stored XSS through log text, denial of service through ingestion/query cardinality, source spoofing, unsafe export, listener misconfiguration, secrets in telemetry, privilege escalation, and audit tampering.

## Authentication

- Bootstrap creates an admin through a one-time secret or CLI flow; there is no shipped default password.
- Passwords are stored with Argon2id using versioned salt/parameters chosen from current OWASP guidance and recalibrated for deployment hardware.
- Browser sessions use at least 256-bit random tokens; only a cryptographic hash is stored. Cookies are `Secure`, `HttpOnly`, `SameSite=Lax/Strict`, scoped narrowly, rotated on login/privilege change, idle- and absolute-expiring.
- State-changing cookie-authenticated requests require origin checks and CSRF tokens.
- Ingestion API keys are scoped, expiring, revocable, shown once, hashed at rest, and distinct from user sessions.
- Login and token endpoints have IP/account-aware throttling without user-enumeration responses.

## Authorization

Policy is centralized around named actions and resources:

```text
logs.view, logs.search, logs.tail, logs.export, query.native
sources.view, sources.manage
searches.manage_own, searches.manage_all
users.manage, settings.manage, retention.manage, audit.view
ingest.logs
```

Default roles map Viewer, Operator, and Admin to actions, but handlers call an authorization service with actor, tenant, action, and resource. Repository/storage calls require tenant scope. UI visibility is not enforcement. Native query and bulk export are separately grantable.

## Query isolation and injection defense

- Portable visual filters remain typed AST nodes until adapter compilation.
- Backend adapters use grammar-aware identifier/literal quoting and property tests with control characters, Unicode, quotes, pipes, comments, and nesting.
- Tenant and time restrictions are trusted structural inputs, not string concatenation supplied by the user.
- Native mode is disabled by default for non-admin roles, audited, time/range/result limited, and executed with least-privileged backend credentials.
- The backend is not exposed to browsers. VictoriaLogs admin/ingest endpoints and ClickHouse DDL/system privileges are not granted to query credentials.
- Query depth, length, operators, range, concurrency, runtime, memory/backend limits, rows, and response bytes are capped.

## Ingestion security

- Every listener has explicit network exposure, tenant/source assignment, parser, maximum frame/request, rate, connections, and trusted-proxy list.
- UDP source addresses are not authenticated. TLS client certificates or agent/API credentials are required when source identity matters.
- Payload tenant, source IP, role, labels, and reserved fields cannot override trusted envelope values.
- Zip/decompression support is disabled or ratio/output-limited to prevent bombs.
- Parser panics are contained at worker boundaries and counted; fuzzing targets framing and parsing.
- Raw binary/log content is encoded safely in JSON and HTML contexts.

## Web and API security

Serve strict headers: CSP without unsafe inline script, HSTS in TLS deployments, `X-Content-Type-Options: nosniff`, restrictive `Permissions-Policy`, `Referrer-Policy`, and frame denial/`frame-ancestors`. CORS is off for same-origin deployment and allowlisted explicitly otherwise. Open redirects are prohibited.

CSV export protects spreadsheet formula cells; filenames and content disposition are sanitized. JSON/NDJSON set correct types. Downloads require authorization throughout stream lifetime and are audited with query hash/count, not sensitive query contents.

## Secrets and encryption

YAML contains secret references, not secrets. Production secrets arrive from mounted files or an orchestrator secret store; environment variables are accepted with a warning about process/environment exposure. Redacted effective config is safe to inspect. TLS is required across untrusted hops and configurable for PostgreSQL/VictoriaLogs. At-rest encryption is supplied by encrypted volumes/object stores initially; application field-level encryption is deferred pending a use case.

## Audit

Audit login outcomes, logout/revocation, user/role/token changes, source lifecycle, storage/retention/settings changes, native query execution, exports, and backup/restore operations. Entries include UTC time, actor, tenant, action, target, result, request ID, origin, and safe before/after hashes. They exclude passwords, tokens, raw logs, and full sensitive queries. PostgreSQL permissions restrict update/delete; off-host forwarding or hash chaining is a production hardening option, so Phase 1 must not overstate tamper-proofness.

## Network and process hardening

- Run non-root with read-only root filesystem, dropped Linux capabilities, no-new-privileges, bounded tmpfs, and explicit writable volumes.
- Prefer host 514 mapped to an unprivileged container port; grant only `CAP_NET_BIND_SERVICE` if direct binding is essential.
- Separate public API, ingestion, backend, database, and monitoring networks/firewall rules.
- Metrics/admin endpoints are independently bindable and authenticated by network/proxy policy.
- Dependencies and container images are pinned, scanned, SBOM-generated, and updated through reviewed automation.

## Verification gates

Threat-model review occurs per phase. CI runs SAST, dependency/license/secret scanning, fuzz/unit tests, OpenAPI checks, image scanning, and security header tests. Release tests include authorization matrix, CSRF, session rotation, injection corpus, XSS fixtures, resource-limit abuse, TLS/mTLS, backup access, and restore drills. A disclosure policy and patch process are required before a production release.
