# Syslog ingestion

## Supported protocols and formats

Phase 1 accepts RFC 3164 and RFC 5424 over UDP and TCP. TCP supports newline and RFC 6587 octet-counted framing. TLS is scheduled for Phase 2/7 and is not silently treated as plain TCP.

The parser extracts priority, facility, severity, timestamp, hostname, application, process ID, message ID, structured data, and message. Original content is retained in `raw_message`. RFC 5424 structured-data parameters use `syslog.sd.<element>.<parameter>` dynamic field names.

Unknown messages accepted by an `auto` source are preserved as `format=unknown` and use receive time. Messages invalid for an explicitly configured parser are dropped and counted. Maximum frame limits apply before normalization.

## Sending examples

UDP with util-linux `logger`:

```bash
logger --server 127.0.0.1 --udp --port 514 "Test syslog message"
```

TCP:

```bash
logger --server 127.0.0.1 --tcp --port 514 "Test TCP syslog message"
```

Devices differ in framing and RFC compliance. Configure a dedicated source/listener when a device requires a fixed parser, timezone, smaller limit, or explicit framing policy.

## Delivery semantics

UDP provides no acknowledgment and may lose datagrams in the network, kernel, or application under overload. TCP provides connection backpressure but standard syslog has no per-event durable acknowledgment. A successful socket write is not proof that an event reached storage. Monitor received, accepted, parsed, stored, retried, and dropped counters independently.

## Troubleshooting

- Check `/ready` for dependency state.
- Inspect `syslogx_messages_parse_error_total` and `syslogx_messages_dropped_total` by bounded reason.
- Confirm Compose maps both `514:1514/udp` and `514:1514/tcp`.
- Confirm host firewalls allow the intended protocol.
- Verify the device clock and listener timezone, especially for RFC 3164.
- Watch queue events/bytes and VictoriaLogs storage latency during bursts.

Do not enable payload logging to troubleshoot production data; use counters, safe error classifications, and controlled synthetic events.
