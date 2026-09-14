# Log data model

Status: proposed canonical contract

## Goals

The model preserves evidence, provides stable cross-format fields, accepts unknown fields, retains provenance, supports future tenants, and maps efficiently to string-oriented or typed backends. Canonical fields have controlled meaning; dynamic fields cannot overwrite them.

## Canonical `LogEntry`

| Field | Domain type | Required | Meaning |
|---|---|---:|---|
| `id` | string | yes | ULID/UUIDv7-like sortable ingestion ID; not a global exactly-once guarantee |
| `tenant_id` | string | yes | Trusted tenant scope assigned before parsing |
| `timestamp` | UTC timestamp ns | yes | Event time, or `received_at` with `timestamp_inferred=true` |
| `received_at` | UTC timestamp ns | yes | Time Syslogx admitted the frame |
| `message` | string | yes | Human-readable event message |
| `raw_message` | bytes/string | policy | Original frame payload after transport framing, before parsing |
| `hostname` | string | no | Sender-declared host after normalization |
| `source_ip` | IP string | no | Transport peer; never overwritten by payload |
| `source_port` | uint16 | no | Transport peer port |
| `facility` | uint8 | no | Syslog facility code 0–23 |
| `facility_name` | string | no | Canonical facility name |
| `severity` | uint8 | no | Syslog severity code 0–7 |
| `severity_name` | string | no | emergency…debug |
| `priority` | uint8 | no | RFC priority; must equal facility*8+severity when present |
| `protocol` | enum | yes | `syslog_udp`, `syslog_tcp`, `syslog_tls`, `http` |
| `format` | enum/string | yes | `rfc3164`, `rfc5424`, `json`, `unknown` |
| `app_name` | string | no | Application/program name |
| `process_id` | string | no | Kept as string because syslog permits non-numeric values |
| `message_id` | string | no | RFC 5424 MSGID or mapped source value |
| `source_id` | string | yes | Stable configured listener/source ID |
| `source` | string | no | Friendly configured source name |
| `source_type` | string | yes | `syslog`, `http_json`, future type |
| `fields` | map<string, Scalar> | yes | Preserved non-canonical source fields |
| `labels` | map<string,string> | yes | Trusted low-cardinality enrichment, not arbitrary payload labels |
| `parse_status` | enum | yes | `parsed`, `partial`, `unknown`, `invalid` |
| `schema_version` | uint16 | yes | Canonical model version |

`Scalar` is string, signed/unsigned integer, decimal/float, Boolean, null, timestamp, IP, or a policy-preserved JSON value in the domain. The VictoriaLogs adapter serializes values canonically to strings and may add companion type metadata only where required. The future ClickHouse adapter retains types.

## Namespace rules

- Canonical names are lowercase `snake_case` and reserved.
- RFC 5424 structured data becomes `syslog.sd.<escaped_element>.<escaped_param>`.
- JSON keys are flattened with `.` by default, with escaping for literal dots and backslashes. Selected subtrees may be preserved as compact JSON.
- Colliding input fields move under `event.<name>`; collision details are counted, not silently discarded.
- Transport-derived attributes use canonical fields; sender-asserted alternatives remain under `event.*` when useful.
- Internal adapter fields use `_syslogx.*`; backend-reserved names such as VictoriaLogs `_time`, `_msg`, and `_stream` are emitted only inside adapters.
- Empty RFC 5424 NILVALUE is absence, not the literal `-`.

## Normalization rules

1. Decode/framing validates maximum bytes before allocation growth.
2. Use RFC timestamp when valid. For RFC 3164 missing year/timezone, apply listener timezone and nearest plausible year; record `timestamp_inferred` and `timestamp_assumption`.
3. Derive priority/facility/severity consistently; invalid values cause partial/invalid status according to parser policy.
4. Normalize severity names to `emergency`, `alert`, `critical`, `error`, `warning`, `notice`, `informational`, `debug`; preserve source level under `event.severity_original` when mapped.
5. Keep source IP from the socket/proxy trust chain. Honor forwarded headers only from configured proxies.
6. Validate UTF-8. Replace invalid sequences for searchable text while preserving original bytes according to raw policy.
7. Enforce configurable limits for event bytes, fields, nesting, key length, value length, and total normalized bytes.

## VictoriaLogs mapping

| Domain | VictoriaLogs |
|---|---|
| `timestamp` | `_time` |
| `message` | `_msg` |
| other canonical fields | same field names, canonical string representation |
| `fields` | flattened top-level fields after namespace validation |
| `labels` | `label.<name>` ordinary fields; only approved subset becomes stream fields |

Initial stream-field candidates are `tenant_id`, `source_type`, `hostname`, and `app_name`, but the deployed set must be benchmarked. `source_ip`, `process_id`, `message_id`, event ID, user ID, request ID, trace ID, and arbitrary dynamic fields are never default stream fields.

## Identity and duplicates

The server generates `id` at admission. HTTP clients may send an idempotency key scoped to tenant/source; a later durable spool can use it to suppress replay duplicates for a finite window. UDP cannot guarantee uniqueness or delivery. Search consumers must tolerate duplicates, late arrival, and event-time ordering ties; ordering uses `(timestamp, received_at, id)` conceptually where the backend can provide it.

## Evolution

Schema changes are additive whenever possible. `schema_version` describes normalization semantics, not database migration state. Renames require dual-read during a documented compatibility window. Adapters own physical mappings. Golden fixtures lock canonical behavior for every parser version.

## Example

```json
{
  "id": "01K...",
  "tenant_id": "default",
  "timestamp": "2026-09-14T14:30:00.000000000Z",
  "received_at": "2026-09-14T14:30:00.024512000Z",
  "message": "VPN tunnel disconnected",
  "hostname": "fw01",
  "source_ip": "10.10.1.1",
  "facility": 20,
  "facility_name": "local4",
  "severity": 4,
  "severity_name": "warning",
  "priority": 164,
  "protocol": "syslog_udp",
  "format": "rfc5424",
  "app_name": "vpn",
  "source_id": "src_syslog_udp",
  "source_type": "syslog",
  "fields": {
    "vendor": "fortinet",
    "device_type": "firewall",
    "vpn_name": "HQ-VPN",
    "interface": "wan1",
    "policy_id": 1234
  },
  "labels": {},
  "parse_status": "parsed",
  "schema_version": 1,
  "raw_message": "<164>1 ..."
}
```
