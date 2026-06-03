# Race conditions in distributed queues — strategy catalog

In distributed systems, **a cluster-wide lock is often a last resort**: high latency, single points of failure, and poor behavior under **network partitions** (split-brain). Production queues usually **structure data and operations** so concurrent workers do not corrupt state — coordination is minimal or pushed to the storage layer.

This document lists common strategies. None are implemented in v1 except **§1** (in-process mutex) and **§1b** (single central owner), which map to this repo.

---

## 1. Centralized mutex (pessimistic locking)

**Idea:** One component holds the canonical queue; all mutations go through it under a lock.

| | |
|---|---|
| **How** | `sync.Mutex` around head/tail updates (in-process) or one queue server process (multi-process). |
| **Race prevention** | Only one mutator at a time. |
| **Pros** | Simple, easy to reason about, strong FIFO on one deque. |
| **Cons** | Does not scale across machines; server is a bottleneck; not a *distributed* lock — just *centralized* state. |
| **This repo** | `memory.MemoryDeque`; `cmd/queued` owns one deque. |
| **Examples** | Your v1 design; many small internal job queues. |

**Note:** A **distributed mutex** (Redis Redlock, etcd lease, etc.) is a different, heavier tool — use when you must coordinate *writers to shared metadata*, not as the default queue implementation.

---

## 2. Optimistic concurrency control (OCC)

**Idea:** Assume conflicts are rare. Read freely; **validate at commit** (version / timestamp). Failed commit → retry.

| | |
|---|---|
| **How** | Each message or partition row has `version`. Lease/update succeeds only if `WHERE version = expected`. |
| **Race prevention** | Second writer’s commit fails when version changed. |
| **Pros** | No long-held locks while workers “think”; good for metadata and lease tables. |
| **Cons** | Retries under contention; requires versioned storage. |
| **This repo** | Not used. Could back a **lease table** beside the deque later. |
| **Examples** | Dynamo-style conditional writes; SQS visibility timeout updates. |

**Queue pattern:** “Claim message” = `UPDATE status PROCESSING WHERE id=? AND version=1` — only one worker wins.

---

## 3. Single-threaded event loop per shard (Redis-style)

**Idea:** Avoid shared mutable state **inside** the shard by handling all commands **sequentially** on one thread (or one goroutine per shard).

| | |
|---|---|
| **How** | Partition queue into shards; each shard has one executor (event loop, actor). |
| **Race prevention** | Concurrent *requests* are serialized at the socket/handler — no lock on data structures if only one thread mutates. |
| **Pros** | Very fast per shard; no lock statements in hot path. |
| **Cons** | Shard hot spots; cross-shard ordering undefined unless you design keys. |
| **This repo** | Analogous to **one** `cmd/queued` process (one goroutine per request still hits one mutex today — could evolve to single worker goroutine + channel). |
| **Examples** | Redis single-threaded core; per-partition actors. |

---

## 4. Atomic compare-and-swap (CAS) at the store

**Idea:** Storage layer exposes **indivisible** check-and-set.

| | |
|---|---|
| **How** | `CAS(key, old, new)` or SQL `UPDATE … WHERE status='AVAILABLE'`. |
| **Race prevention** | Only one transition wins; others see failure and retry or skip. |
| **Cons** | Needs store with atomic primitives; design schema around states. |
| **This repo** | Not used (in-memory list, no external store). |
| **Examples** | etcd, Consul, Cassandra lightweight transactions. |

**Queue pattern:** Claim = atomic state transition `AVAILABLE → PROCESSING`, not “read then write” in two steps without guard.

---

## 5. Append-only immutable log (Kafka-style)

**Idea:** Producers **append**; consumers advance an **offset**. The log segment is not mutated in place.

| | |
|---|---|
| **How** | Messages are records in an ordered log; “pop” becomes “read and commit offset” (often with consumer groups). |
| **Race prevention** | No in-place delete races on shared cells; contention moves to offset commits (OCC/CAS). |
| **Pros** | Replay, high throughput, many consumers with coordination. |
| **Cons** | Not a classic in-memory deque; retention, compaction, consumer lag. |
| **This repo** | Different product shape — would be a **separate track**, not `MemoryDeque` with pointers. |
| **Examples** | Apache Kafka, Pulsar. |

**Fit:** Streaming, event sourcing, re-play — not the same as O(1) doubly-linked deque in RAM.

---

## 6. Conflict-free replicated data types (CRDTs)

**Idea:** Replicas update **without** central locking; math guarantees **eventual convergence** to the same state.

| | |
|---|---|
| **How** | CRDT sets/counters/maps merge concurrent updates deterministically. |
| **Race prevention** | By construction — no “lost update” in the CRDT sense. |
| **Pros** | Geo-distribution, partition tolerance for *certain* data types. |
| **Cons** | Strict global FIFO is hard; semantics differ (LWW, set union, etc.). |
| **This repo** | Not used. |
| **Examples** | Riak-style sets; collaborative structures; some “queue depth” counters — rarely strict job FIFO. |

**Fit:** Loose ordering, membership, metrics — not primary choice for strict job queue FIFO.

---

## 7. Partitioning (sharding)

**Idea:** Split traffic by key (`user_id`, `tenant`, `shard_id`) so each partition has **independent** concurrency domain.

| | |
|---|---|
| **How** | `deque:tenant-42` → one owner or one shard; workers only compete inside a partition. |
| **Race prevention** | Smaller critical sections; less cross-tenant contention. |
| **Pros** | Horizontal scale; aligns with event-loop-per-shard or OCC per partition. |
| **Cons** | Cross-partition ordering undefined; hot keys. |
| **This repo** | Mode C in README (broker per entity); multiple `MemoryDeque` or multiple server instances **with routing** — not v1. |
| **Examples** | Kafka partitions; SQS FIFO per message group; Redis hash tags. |

---

## 8. Leasing, visibility timeout, and idempotency (operational layer)

**Idea:** Race-free *structure* is not enough — you need **delivery semantics**.

| | |
|---|---|
| **How** | Pop = lease with timeout; ack/nack; idempotency keys on handlers. |
| **Race prevention** | Duplicate delivery handled in app; ambiguous timeout handled with dedup store. |
| **This repo** | Documented as non-goals for v1 (at-most-once after pop). |
| **Examples** | SQS visibility timeout; RabbitMQ acks. |

---

## 9. Consensus and coordination services (when you need strong agreement)

**Idea:** Use Raft/Paxos (etcd, ZooKeeper) for **small, critical metadata** — leader election, barrier, offset — not for every byte of every message.

| | |
|---|---|
| **Race prevention** | Linearizable updates to coordination keys. |
| **Cons** | Latency; operational complexity. |
| **This repo** | Out of scope for v1. |

---

## Summary table (design checklist)

| Strategy | When to use | Real-world flavor | In this repo (v1) |
|----------|-------------|-------------------|-------------------|
| Centralized mutex / single owner | Learning, single broker, strict FIFO on one deque | Small queue server | **Yes** — `memory` + `cmd/queued` |
| Optimistic concurrency | Versioned leases, metadata DB | Conditional writes | No — doc / future |
| Event loop per shard | High throughput per partition | Redis-like shard | Partial analogy — one server |
| Atomic CAS | Claim bits in distributed store | etcd, DB conditions | No |
| Append-only log | Stream, replay, scale-out consumers | Kafka, Pulsar | No — different model |
| CRDTs | Eventual consistency, loose ordering | Geo replicas | No |
| Partitioning | Multi-tenant / scale | Sharded queues | Concept only (README Mode C) |
| Leasing + idempotency | Safe retries, at-least-once | SQS, Rabbit | Documented limitation only |
| Consensus | Leader, barriers, offsets | etcd, ZK | Out of scope |

---

## Strict FIFO vs pub/sub (chooser)

| You need | Lean toward |
|----------|-------------|
| Strict FIFO on one lane | Central owner (v1) or partitioned FIFO (SQS FIFO group) |
| Max throughput, replay, many consumer groups | Append-only log |
| Simple broker, cache-speed per shard | Partition + event loop |
| Geo-replicas, loose ordering | CRDTs (careful) |
| Coordinate workers without locking the queue body | OCC / CAS on **lease** rows |

See [`design-chooser.md`](design-chooser.md) for a decision flowchart and [`this-project.md`](this-project.md) for how far this repository goes in v1.
