# Design chooser — strict FIFO vs scalable pub/sub

Use this when deciding **which race-strategy family** fits a queue design — including extensions beyond v1 of **distributed-deque**.

---

## Step 1 — What is your primary guarantee?

| Priority | Direction |
|----------|-----------|
| **Strict FIFO** on one lane | Central owner (v1), or **partitioned FIFO** (one ordered stream per `group_id`) |
| **High throughput**, replay, many consumer types | **Append-only log** |
| **Low latency** per key, cache-like speed | **Partition + single-threaded shard** |
| **Geo replicas**, loose ordering OK | **CRDTs** (validate semantics carefully) |
| **Coordinate workers** without locking message bodies | **OCC / CAS on lease metadata** |

---

## Step 2 — Where does state live?

```text
                    ┌──────────────────────┐
                    │  Canonical messages  │
                    └──────────┬───────────┘
                               │
         ┌─────────────────────┼─────────────────────┐
         ▼                     ▼                     ▼
   In-memory deque      Append-only log        External store
   (this repo v1)       (Kafka track)          (SQL + OCC/CAS)
```

| State location | Typical strategies |
|----------------|-------------------|
| One process RAM | Mutex (v1) |
| One server RAM + HTTP | Central owner (v1 `cmd/queued`) |
| Durable log | Append-only + offset commits |
| Row per message | OCC, CAS, leasing |

---

## Step 3 — Delivery semantics

| Need | Implies |
|------|---------|
| Fire-and-forget pop | v1 at-most-once (current) |
| Worker crash after pop | **Lease + ack** or **idempotent** handlers |
| Exactly-once processing | Out of scope here — ledger + dedup (not mutex alone) |

---

## Step 4 — Match to checklist

| Strategy | When to use it | Example systems |
|----------|----------------|-----------------|
| **Partitioning + event loop** | High throughput per shard | Redis lists, per-shard actors |
| **Append-only log** | Streaming, replay, fan-out | Kafka, Pulsar |
| **Atomic CAS / OCC** | Claims on shared metadata | etcd, conditional DB updates |
| **Central mutex / owner** | Learning, single broker, tests | **This repo v1** |
| **CRDTs** | Eventual merge without central lock | Specialized replicas |

---

## Step 5 — Fit for *this* repository

| Your goal | Stay on v1 | Read track |
|-----------|------------|------------|
| Learn deque + HTTP + errors | Yes | `tracks/01-mutex-central-owner.md` |
| Compare Redis/Kafka mentally | Docs only | `02`–`06` |
| Experiment with sharding | Future layout | `07-partitioning.md`, `future-layout.md` |
| Build Kafka in Go | Different project / track 05 | `05-append-only-log.md` |

---

## ASCII flow (quick)

```text
Need strict FIFO on one queue?
  yes → Central owner (v1) OR partitioned FIFO shard
  no  → Continue

Need replay / event stream?
  yes → Append-only log track
  no  → Continue

Need max RPS per key with simple model?
  yes → Partition + event-loop shard
  no  → Continue

OK with eventual consistency?
  yes → CRDT track (read warnings)
  no  → OCC/CAS on lease rows + central or sharded storage
```

---

## Related

- [`race-condition-strategies.md`](race-condition-strategies.md)
- [`this-project.md`](this-project.md)
- [`../deque-guide.md`](../deque-guide.md) §3.1 ordering modes
