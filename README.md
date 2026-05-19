# distributed-deque

A distributed **deque** (double-ended queue) for Go, backed by Redis. The library exposes a single interface with two deployment modes—**Redis Lists** for low-latency ephemeral workloads and **Redis Streams** for durable, ack-aware processing—so you can choose the right engine without changing application semantics at the API boundary.

Both modes support **multi-process** coordination (many clients, one shared deque per key) and **multi-region** topologies (primary + DR, or regional partitioned deques)—see [Distributed coordination](#distributed-coordination), [Replication & failover](#replication--failover), [Multi-region](#multi-region), and [Production considerations](#production-considerations).

---

## Repository naming

| | |
|---|---|
| **Initial proposal** | `distributed-queue` |
| **Final name** | `distributed-deque` |

A standard **queue** implies FIFO (first-in, first-out) semantics. This system supports **bidirectional** insertion and removal at both ends (head and tail). The term **deque** matches that behavior, keeps the scope precise, and makes the library easier to discover as an open-source component.

---

## What is a deque?

A **deque** sits between a queue (FIFO) and a stack (LIFO). It supports four atomic operations:

| Operation | Aliases | Description |
|-----------|---------|-------------|
| `PushFront` | Prepend | Insert at the **head** |
| `PushBack` | Append | Insert at the **tail** |
| `PopFront` | Shift | Remove from the **head** |
| `PopBack` | Pop | Remove from the **tail** |

---

## Architecture

One Go deque interface is implemented by two Redis-backed engines. Pick the variant that fits your deployment; both remain distributed via Redis.

```
                    ┌─────────────────────┐
                    │   Deque interface   │
                    │  (Go, distributed)  │
                    └──────────┬──────────┘
                               │
              ┌────────────────┴────────────────┐
              ▼                                 ▼
    ┌──────────────────┐              ┌──────────────────┐
    │    ListDeque     │              │   StreamDeque    │
    │  (Redis Lists)   │              │ (Redis Streams)  │
    └──────────────────┘              └──────────────────┘
```

### Mode A: `ListDeque` (Redis Lists)

**Commands:** `LPUSH`, `RPUSH`, `LPOP`, `RPOP` (and blocking variants `BLPOP`, `BRPOP`).

| | |
|---|---|
| **Characteristics** | Very low latency, small memory footprint, ephemeral lifecycle |
| **Best for** | Volatile, high-throughput task routing where items are processed quickly and discarded after retrieval |

Use this mode when you need a lightweight, distributed deque and do not require stream-style persistence or acknowledgement.

### Mode B: `StreamDeque` (Redis Streams)

**Commands:** `XADD`, `XACK`, plus custom multi-index read indexing for bidirectional processing.

| | |
|---|---|
| **Characteristics** | Append-only log layout, persistence, built-in message acknowledgement |
| **Best for** | Event sourcing, audit logs, and resilient processing with consumer groups for at-least-once delivery |

Use this mode when durability, replay, and explicit ack semantics matter more than the minimal footprint of list-based storage.

---

## Choosing a mode

| Concern | Prefer **ListDeque** | Prefer **StreamDeque** |
|---------|----------------------|------------------------|
| Latency | Lower | Higher (log + ack overhead) |
| Durability | Ephemeral | Persistent append-only log |
| Delivery guarantees | Fire-and-forget pop | At-least-once with `XACK` / consumer groups |
| Typical use | Task queues, fast routing | Events, audits, replayable pipelines |

---

## Distributed coordination

**Distributed** means many application processes (on one host or many) share **one logical deque** whose authoritative state lives in **Redis**, not in process memory.

```
  Node A (worker)     Node B (API)      Node C (worker)
        │                  │                    │
        └──────────────────┼────────────────────┘
                           │
                           ▼
                    ┌──────────────┐
                    │    Redis     │  ← one list or stream key per deque
                    └──────────────┘
```

Within a single Redis deployment (standalone, Sentinel, or Cluster), one deque key is served by one primary shard. Extra Redis nodes provide replication and failover—they do not split one list/stream across the cluster unless you deliberately partition with multiple keys at the application layer.

---

## Replication & failover

The library does **not** dual-write to primaries and replicas. Clients send commands to the **current primary** (or a Sentinel/Cluster-aware client that tracks it). Redis replicates data to replicas in the background.

```
  App (Push/Pop)  ──►  Redis PRIMARY  ──replication──►  REPLICA(s)
```

| Topic | Behavior |
|-------|----------|
| **Writes** | Always to the primary for that deque key |
| **Replicas** | Read-only copies for durability and promotion on failover |
| **Failover** | Sentinel or Cluster promotes a replica; clients reconnect to the new primary |
| **Lag** | Unreplicated writes can be lost if the primary fails before replication catches up |

**ListDeque:** pops and pushes replicate as ordinary Redis list mutations. **StreamDeque:** entries, consumer groups, and acks replicate with standard Redis stream replication; after failover, expect at-least-once delivery and possible redelivery of in-flight messages.

Optional read-from-replica is a deployment choice (lower load on primary, risk of stale reads). It is not a substitute for writing to a replica.

---

## Multi-region

Multi-region extends the same model: **each region needs a clear source of truth**, and cross-region links are **async and high-latency** compared to local Redis. The library targets explicit regional modes rather than implying one magically consistent global deque over WAN distance.

### Deployment patterns

| Pattern | How it works | Best for |
|---------|----------------|----------|
| **Primary + DR (active–passive)** | One **home region** holds the writable deque; a **standby region** receives async replication (Redis replica across regions, managed service DR, or periodic backup). Failover promotes DR; apps switch region. | Disaster recovery, regulatory “warm” standby |
| **Regional deques (active–active, partitioned)** | Each region has its **own** Redis and deque key(s). Work is **routed by region** (tenant locale, geo hash, shard id). No cross-region pop on every operation. | Low-latency local workers, global product with regional isolation |
| **Global primary (single writer region)** | All regions write/read one primary in a chosen region. | Rare; simple mentally but cross-region RTT on every operation |
| **Follower reads (optional)** | Local replica serves **read-only** deque inspection; **writes and pops** still go to the primary (local or remote). | Metrics, peek, admin— not strict linearizable pop semantics |

```
  Region US (primary)                    Region EU (DR replica)
  ┌─────────────────┐                  ┌─────────────────┐
  │ Redis primary   │ ── async repl ─► │ Redis replica   │
  │ deque key       │                  │ (promote on DR) │
  └────────▲────────┘                  └─────────────────┘
           │
    US apps (read/write)

  Region US              Region EU              Region APAC
  ┌──────────────┐       ┌──────────────┐       ┌──────────────┐
  │ Redis + deque│       │ Redis + deque│       │ Redis + deque│
  │  (shard US)  │       │  (shard EU)  │       │  (shard APAC)│
  └──────▲───────┘       └──────▲───────┘       └──────▲───────┘
         │                      │                      │
    local workers           local workers           local workers
         (partitioned / routed work — no shared pop across regions)
```

### What the library does *not* do by default

- **Dual-write from the app** to a primary in US and a primary in EU on every `PushBack`—that creates split-brain, partial failures, and duplicate or lost items unless you add idempotency and conflict rules.
- **Treat a replica as a second writable deque**—replicas stay read-only until promotion.
- **Hide WAN latency**—a single global deque with one primary cannot be as fast as regional Redis for local producers and consumers.

Multi-region support in this project means **configurable regional backends**, **failover targets**, and (where needed) **partitioning helpers**—not transparent synchronous replication on every operation.

### Recommended mode by pattern

| Pattern | Prefer | Why |
|---------|--------|-----|
| Primary + DR | **StreamDeque** | Durable log, acks, and replay after regional failover; easier to reconcile in-flight work |
| Regional partitioned deques | **ListDeque** or **StreamDeque** | Lists for volatile per-region task routing; streams when each region needs audit or acked delivery |
| Cross-region event fan-out | **StreamDeque** (+ optional outbox) | Append events in the source region; async consumers or replication tooling propagate copies |

### Consistency expectations

| Guarantee | Single region | Multi-region |
|-----------|---------------|--------------|
| **Linearizable pop/push** | Yes (against local primary) | Yes **within** a region’s primary |
| **Cross-region** | N/A | **Eventual** replication; bounded loss possible on primary failure before repl |
| **After regional failover** | Promoted replica may lag | Same; **StreamDeque** + idempotent handlers recommended |
| **Active–active same key** | N/A | Not supported without external conflict resolution (e.g. vendor active-active or app-level CRDTs) |

### Configuration surface (planned)

Multi-region behavior will be expressed through client configuration, for example:

- **Region** — logical name (`us-east`, `eu-west`) for metrics and routing.
- **Role** — `primary`, `replica`, or `regional-primary` (partitioned active–active).
- **Endpoints** — Redis URL(s) for the local deployment; optional **DR endpoint** for failover clients.
- **Deque key** — base key or sharded keys (`deque:{region}` / `deque:{shard}`).
- **Failover** — Sentinel/Cluster URLs, or explicit promotion callback when using managed Redis DR.

Example (illustrative, API not final):

```go
// Active–passive: US writes; EU is DR (reads optional, promote on failover)
us, _ := distributeddeque.NewStreamDeque(distributeddeque.Config{
    Region:    "us-east",
    Role:      distributeddeque.RolePrimary,
    RedisURL:  os.Getenv("REDIS_US_PRIMARY"),
    Key:       "orders",
})

eu, _ := distributeddeque.NewStreamDeque(distributeddeque.Config{
    Region:    "eu-west",
    Role:      distributeddeque.RoleReplica,
    RedisURL:  os.Getenv("REDIS_EU_REPLICA"),
    Key:       "orders",
    ReadOnly:  true,
})

// Regional partition: each region owns its deque
euLocal, _ := distributeddeque.NewListDeque(distributeddeque.Config{
    Region:   "eu-west",
    Role:     distributeddeque.RoleRegionalPrimary,
    RedisURL: os.Getenv("REDIS_EU"),
    Key:      "tasks:eu",
})
```

### Operating multi-region safely

1. **Prefer one writable primary per deque key** unless using explicit regional partitioning.
2. **Use StreamDeque** when failover or cross-region copy must not silently drop acked work.
3. **Design handlers to be idempotent**—failover and replication lag can redeliver items.
4. **Measure replication lag** between regions before relying on DR RPO targets.
5. **Test promotion**—clients must discover the new primary and stop writing to the old one.

For vendor-specific active-active Redis (multi-master at the datastore layer), treat it as an advanced deployment: the deque interface stays the same, but consistency guarantees follow the provider’s documentation, not single-primary Redis semantics.

---

## Production considerations

Checklist for building and operating a production-ready application on top of this library. Items marked **(planned)** are intended library or documentation deliverables as the implementation matures.

### Delivery semantics

Define a written contract per workload—do not assume FIFO queue semantics alone when using both ends of the deque.

| Topic | ListDeque | StreamDeque |
|-------|-----------|-------------|
| **Typical guarantee** | At-most-once if the worker crashes after pop | At-least-once with `XACK` |
| **Duplicates** | Uncommon; possible after retry when pop result was unknown | Expected after retry, failover, or reclaim |
| **Lost work** | Possible if the process dies after pop | Possible if ack happens too early or entries are trimmed |
| **Ordering** | Rough FIFO per end; not global when mixing head/tail ops | Per stream; consumer groups affect “who is next” |

**Idempotency keys** — Producers attach a stable idempotency key per logical message. Consumers record processed keys in a **dedup store** (Redis `SET` with short TTL, or a database) so retries and redeliveries do not repeat side effects.

**Process then ack** — For StreamDeque, never `XACK` until side effects are durable (database commit, downstream publish, etc.). Acking first loses at-least-once protection.

**Bidirectional pops and consumer pools** — Decide whether the same consumer pool may call both `PopFront` and `PopBack`. A common split:

| Role | Allowed ops | Rationale |
|------|-------------|-----------|
| Producers / API | `PushBack` (and sometimes `PushFront` for priority) | Enqueue only |
| Workers | `PopFront` only | Single consumption lane |
| Priority path | Dedicated workers on `PopFront` after `PushFront` | Avoids starving normal tail traffic |

Mixing unprioritized head prepends with tail consumers without rules can cause **priority inversion** (endless prepends starve tail pops).

---

### Reliability: clients, Redis, and shutdown

| Concern | What to account for |
|---------|---------------------|
| **Timeouts** | Every command: connect, read, write. Blocking pops (`BLPOP`, `XREAD BLOCK`) need a **max wait** and respect `context` cancellation. **(planned)** |
| **Retries** | Only for **safe** operations—e.g. push with an idempotency key. Do **not** blindly repush after an unknown pop; do **not** retry a successful pop. |
| **Circuit breaker** | When Redis is unhealthy, stop hammering, shed load, and alert. **(planned)** optional wrapper or documented integration pattern. |
| **Connection pooling** | Configure pool size, max idle connections, and dial timeout (e.g. go-redis). Avoid per-operation dial. |
| **Failover** | Rediscover primary via Sentinel/Cluster; drain in-flight work; handle `READONLY` after replica promotion; stop writing to the old primary. |
| **Graceful shutdown** | Cancel blocking workers; wait for in-flight handlers; for streams, reclaim the **pending entry list (PEL)** with `XAUTOCLAIM` / `XCLAIM` for stragglers. **(planned)** helpers |

**Half-open failures** — A command may succeed on the server while the client times out. Treat pop results as unknown until verified; use idempotency on the consumer side.

---

### Poison messages, DLQ, backpressure, and visibility

| Concern | Mitigation |
|---------|------------|
| **Poison message** | Handler always fails; without a **dead-letter queue (DLQ)** or dead-letter stream, workers spin forever. |
| **Retries** | Max attempts + exponential backoff; then move to DLQ with error metadata (last error, stack hash, attempt count). **(planned)** |
| **DLQ replay** | Document who may replay, manual vs automated, and idempotency on replay. |
| **Backpressure** | Unbounded `PushBack` fills Redis memory. Use `MAXLEN` on streams, app-side max length on lists, **reject when depth > threshold**, or rate-limit producers. **(planned)** depth check helpers |
| **Visibility timeout** | Streams: entries stay in the PEL until acked or idle long enough to **claim** (`XAUTOCLAIM`). Prevents lost work when a worker crashes mid-handler. |

---

### Payloads: serialization, size, and evolution

| Concern | Guidance |
|---------|----------|
| **Serialization** | JSON, Protobuf, or a versioned envelope `{ "schema_version": 1, "payload": ... }`. |
| **Size limits** | Redis values have practical size limits; store large blobs in object storage (S3, etc.) and enqueue a **pointer** (URL + checksum). |
| **Schema evolution** | Forward-compatible readers; explicit migration for breaking changes. |
| **Compression** | Optional for large payloads; trade CPU vs memory and network. |

Do not put **secrets** in deque payloads unless encrypted at the application level.

---

### Security

| Concern | Guidance |
|---------|----------|
| **Transport** | TLS to Redis; private network / VPC; no public Redis endpoints. |
| **ACLs** | Least privilege—application user limited to list/stream commands on prefixed keys (e.g. `deque:*`). |
| **Auth rotation** | Support password/secret rotation without hard downtime (dual-credential window or managed rotation). |
| **Multi-tenant isolation** | Key prefix per tenant (`deque:{tenant}:tasks`); optional separate Redis DB or instance for large tenants. |

---

### Redis operations (production SRE)

| Risk | Mitigation |
|------|------------|
| **Memory pressure** | `maxmemory` policy, monitor length/bytes per key, stream trimming policy |
| **Persistence** | AOF/RDB where durability is required; accept possible loss of last seconds on crash |
| **Single hot key** | One deque key = one shard hotspot; scale via **sharding** multiple keys |
| **`KEYS` / `FLUSH`** | Ban in production; use `SCAN` with key prefix |
| **Eviction** | `allkeys-lru` can **silently drop queue items**—usually wrong for task queues; prefer `noeviction` or policies that fail writes when full |
| **Version drift** | Pin Redis version; test list/stream behavior on upgrades |
| **Maintenance** | Failover drills, backup/restore tests; understand `CLIENT PAUSE` impact on blocking operations |

**Runbooks** — Document behavior when Redis is entirely unavailable (degrade feature vs alternate queue vs fail closed).

---

### Deque-specific application rules

Document these in your service, not only in Redis configuration:

| Decision | Notes |
|----------|-------|
| **Who pops which end?** | e.g. priority lane at head (`PopFront` after `PushFront`), normal work at tail (`PopBack` / `PopFront` from tail side only). |
| **Priority inversion** | Unbounded `PushFront` can starve tail consumers—cap priority rate or use a separate deque key. |
| **Concurrent consumers** | Many workers popping the same end is atomic; ordering is not globally “fair” across workers. |
| **Peeking** | `LRANGE` / `XRANGE` without remove—for ops and dashboards, not the main processing path. |
| **Empty deque** | Distinguish `ErrEmpty`, timeout, and connection errors in the API. **(planned)** typed errors |
| **Blocking vs polling** | Prefer `BLPOP` / `XREAD BLOCK` over tight loops to avoid CPU burn. |

**Exactly-once** is not provided by Redis lists/streams alone; use an outbox, database transaction, or external coordinator if required.

---

### Observability

Minimum instrumentation for production:

| Signal | Examples |
|--------|----------|
| **Metrics** | Depth (length), push/pop rate, latency histograms, errors by type, replication lag, PEL size (streams), DLQ rate, consumer lag |
| **Tracing** | Trace ID in the message envelope; span per push → process → ack |
| **Structured logs** | Region, role, deque key, message id, attempt count |
| **Alerts** | Depth above SLO, lag growing, error rate, Redis memory, failover events |

**SLOs** — Example: “95% of tasks start processing within 30s of enqueue.” Build dashboards per region and shard.

**(planned)** Optional metrics/tracing hooks on the deque client.

---

### Library roadmap (production-related)

As the Go implementation lands, prioritize:

1. Typed errors (`ErrEmpty`, `ErrReadOnly`, `ErrTimeout`, `ErrClosed`)
2. `context.Context` on all operations
3. Health check helper (`PING` + optional depth probe)
4. StreamDeque: consumer groups, `XAUTOCLAIM`, DLQ routing, trim policy
5. ListDeque: blocking pop with shutdown and max wait
6. Documented non-goals: exactly-once, built-in scheduler, cross-region synchronous consistency

---

## Status

This repository is under active development. API details, installation, and examples will be added as implementations land.
