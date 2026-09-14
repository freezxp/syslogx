#!/bin/sh
set -eu

api_url=${SYSLOGX_SMOKE_API_URL:-http://127.0.0.1:8080}
syslog_host=${SYSLOGX_SMOKE_SYSLOG_HOST:-127.0.0.1}
syslog_port=${SYSLOGX_SMOKE_SYSLOG_PORT:-514}
token="syslogx-smoke-$(date +%s)"

curl -fsS "$api_url/ready" >/dev/null
logger --server "$syslog_host" --udp --port "$syslog_port" "$token"

attempt=0
while [ "$attempt" -lt 20 ]; do
  if curl -fsS "$api_url/api/v1/system/logs/recent?limit=100" | grep -F "$token" >/dev/null; then
    echo "smoke test passed: $token"
    exit 0
  fi
  attempt=$((attempt + 1))
  sleep 1
done

echo "smoke test failed: event was not searchable within 20 seconds" >&2
exit 1
