# Storage backend comparison

Status: accepted recommendation for initial implementation  
Scope: VictoriaLogs versus self-managed ClickHouse for the Syslogx log data plane

## Recommendation

Use **VictoriaLogs in Phase 1** behind a backend-neutral port. It most directly supplies the product primitives Syslogx needs: schemaless flat fields, indexing across fields, full-text LogsQL, field names/values, facets, hit histograms, statistics endpoints, streaming query responses, and live tail. Its single-binary/single-node profile keeps the first deployment understandable.

Keep **ClickHouse as the first additional adapter**. Prefer it when SQL analytics, typed columns, materialized views, custom rollups, multi-table correlation, mature backup tooling, or an existing ClickHouse operating practice outweigh the extra schema/index/query engineering.

This is not a universal performance verdict. Vendor-independent benchmarks using Syslogx's event distributions and queries are required before scale claims or backend changes.

## Comparison

| Dimension | VictoriaLogs | ClickHouse |
|---|---|---|
| Ingestion | Purpose-built log ingestion and HTTP/syslog integrations; straightforward batching | Extremely high insert throughput when batches and part counts are controlled; poor insert shape creates merge pressure |
| Recent log query | Log-native filters and stream pruning; strong fit | Strong when sort/primary key matches access pattern; otherwise parallel scans and skipping indexes |
| Full-text | Native LogsQL word/phrase/prefix search; all fields indexed | Fast scans plus optional text/token/Bloom indexes; requires deliberate design and version validation |
| Arbitrary fields | Flat, schema-free, string-valued fields; automatic indexing | JSON/Dynamic/Map or raw JSON plus promoted columns; more modeling choices and tuning |
| Typed values | Values are strings; type-aware semantics need query conversion/conventions | Rich native types and SQL comparisons |
| High cardinality | Fine for ordinary fields; dangerous as stream fields | Usually manageable as columns; primary/order/index choices determine cost |
| Field discovery/facets | Dedicated field, value, and facet APIs | Implement with JSON path discovery/system metadata and GROUP BY; may be expensive without projections/materialized views |
| Time range | Native `_time` model and time filters | Excellent with time partitioning and appropriate ordering key |
| Aggregations | LogsQL stats/hits APIs cover log dashboards | Major strength: SQL, vectorized aggregation, materialized views, projections |
| Compression/storage | Log-oriented column storage and stream-aware compression | Excellent columnar compression; codecs and sort order provide extensive control |
| Retention | Simple age/disk-based daily-partition retention | Flexible row/column TTL, tier moves, and rollups; merge timing is eventual |
| Single-node operation | Very low component count | More database administration and tuning surface |
| Horizontal scale | `vlinsert`/`vlselect`/`vlstorage`; simple sharding, but cluster sharding is not replication | Distributed tables, replicated MergeTree, Keeper; powerful but significantly more complex |
| Backup/restore | Partition snapshots plus external copy/rclone; integrated backup-manager work remains on roadmap | Native `BACKUP`/`RESTORE` to configured destinations plus broad ecosystem |
| Resource controls | Query concurrency/duration/memory controls and streaming backpressure | Extensive per-user/query settings, quotas, workload controls |
| Ecosystem | Focused log ecosystem, LogsQL, Grafana plugin | Broad SQL ecosystem, OTel patterns, many clients and analytics integrations |
| Portability cost | LogsQL and string-field semantics | SQL/schema/order-key/materialized-view semantics |

## Important details

### VictoriaLogs strengths

- A log entry can contain arbitrary fields; fields are automatically indexed. This closely matches dynamic field discovery.
- Dedicated `/field_names`, `/field_values`, `/facets`, `/hits`, statistics, query, and tail endpoints reduce application-side scanning.
- Query output streams and respects downstream read speed, which supports cancellation and bounded export.
- Daily partitions make simple retention predictable. Single-node to cluster migration is documented.
- Stream fields improve compression and queries when low-cardinality, stable identity fields are selected correctly.

### VictoriaLogs limitations and mitigations

- All ordinary field values are strings. Store canonical numeric/time values consistently and expose operator capabilities; never silently perform lexical comparison where numeric comparison was requested.
- Field/stream cardinality mistakes can consume resources. Default stream fields to stable `tenant_id`, `hostname`, `app_name`, and `source_type` only after cardinality testing; never use trace ID, user ID, message ID, or arbitrary source IP by default.
- A log entry has a documented field-count ceiling and nested JSON is normally flattened. Syslogx imposes a lower configurable safety limit and supports preservation of selected JSON subtrees.
- Cluster nodes are sharded, not replicated by default. Production HA and backups need an explicit topology and restore drill.
- Snapshot orchestration is less integrated than ClickHouse native backup workflows. Phase 1 documents volume snapshots; production profiles automate verified off-host copies.

### ClickHouse strengths

- Typed schemas, SQL, vectorized aggregation, materialized views, projections, and TTL rollups excel for analytics and dashboards.
- Columnar compression and highly parallel scans are proven for very large observability data sets.
- Flexible JSON approaches allow a raw event plus selected promoted columns, or a dynamic structure with discovered paths.
- Replicated MergeTree and Keeper provide mature building blocks for self-managed replicated clusters.
- Native backup/restore and object-storage options provide a broader operational toolkit.

### ClickHouse limitations and mitigations

- ClickHouse is an engine, not an out-of-the-box log product. Syslogx must design schema, text indexes, field discovery, tail polling/streaming, and safe SQL generation.
- Query speed depends strongly on partition/order keys and schema. A wrong early choice causes broad scans or migrations.
- Highly dynamic data needs trade-offs: `JSON`/Dynamic paths, `Map`, raw JSON, or promoted columns have different costs and version constraints.
- Many tiny inserts create too many parts. The adapter must enforce meaningful batches and monitor merges/parts.
- Distributed, replicated operation adds Keeper, topology, shard/replica, DDL, balancing, and upgrade complexity.

## Backend abstraction rules

The common API promises semantics, not syntax. The domain query AST supports only capabilities with conformance tests. Adapters publish:

- supported predicates and value types;
- full-text behavior/tokenization notes;
- tail support and expected delay;
- aggregation functions and bucket constraints;
- cursor consistency level;
- retention management mode;
- native dialect name and availability.

Mandatory tenant/time constraints are passed as structured adapter inputs and cannot be overridden by native text. Advanced native queries are labeled `logsql` or `clickhouse_sql`, permission-gated, capped, audited, and non-portable.

## Decision triggers for ClickHouse

Prototype and benchmark the ClickHouse adapter when one or more is true:

1. Dashboard or cross-field aggregation latency misses objectives after VictoriaLogs tuning.
2. Typed numeric analytics and long-lived materialized rollups become core product features.
3. Customers already operate ClickHouse or require its backup/replication ecosystem.
4. Correlation with traces or non-log relational/analytical data is required.
5. VictoriaLogs field, query-language, HA, or retention constraints block a documented requirement.

Migration is dual-write/backfill/compare/cutover, never an in-place flag flip. Saved visual queries remain portable; native saved queries remain tied to their dialect.

## Benchmark plan

Compare both engines using identical generated corpora at 1K, 10K, 50K, and 100K events/s, with short/bursty messages, large JSON, low/high field cardinality, and late timestamps. Record accepted/persisted/dropped counts, CPU, RSS, disk bytes, write amplification, p50/p95/p99 ingest latency, and query latency for recent text, selective field, high-cardinality field, facets, top-N, histogram, and export. Run cold/warm cache and concurrent ingestion/query cases for at least 30 minutes plus a 24-hour soak.

## Sources

- [VictoriaLogs key concepts](https://docs.victoriametrics.com/victorialogs/keyconcepts/)
- [VictoriaLogs querying APIs](https://docs.victoriametrics.com/victorialogs/querying/)
- [VictoriaLogs retention, snapshots, and backup](https://docs.victoriametrics.com/victorialogs/)
- [VictoriaLogs cluster architecture and replication caveat](https://docs.victoriametrics.com/victorialogs/cluster/)
- [VictoriaLogs metrics](https://docs.victoriametrics.com/victorialogs/metrics/)
- [ClickHouse observability guidance](https://clickhouse.com/docs/guides/use-cases/observability/build-your-own/introduction)
- [ClickHouse JSON guidance](https://clickhouse.com/docs/guides/clickhouse/data-formats/json/intro)
- [ClickHouse TTL](https://clickhouse.com/docs/concepts/features/operations/delete/ttl)
- [ClickHouse backup and restore](https://clickhouse.com/docs/concepts/features/backup-restore/overview)
