# How this repository maps to distributed strategies

This project is intentionally **small and visible**. It teaches **ownership** and **HTTP boundaries** before adopting heavier distributed patterns.

---

## What we implement today (v1)

```text
┌─────────────────────────────────────────────────────────┐
│  cmd/queued (one OS process)                            │
│    └── memory.MemoryDeque                               │
│          └── sync.Mutex  ← strategy §1 (centralized)    │
│    HTTP: POST /push  GET /pop                          │
└─────────────────────────────────────────────────────────┘
         ▲
         │  curl / HTTP clients (any process)
         │
   Worker terminals (no deque in their heap)
```

| Concern | v1 choice |
|---------|-----------|
| Race prevention on deque structure | **Mutex** on linked list |
| Multi-process | **Single owner** server — not a distributed lock |
| Client | **HTTP** (`curl`); no `remote` Go package |
| Empty vs down | HTTP **204** vs connection errors (client responsibility) |
| Delivery | **At-most-once** after successful pop — no lease/ack |
| Ordering | Mode A / B / C in README — your test or deployment policy |

This is **not** “wrong” for learning — it is the same **central broker** pattern many systems use before sharding.

---

## What we do *not* implement (documented alternatives)

| Strategy | Why not in v1 | Where documented |
|----------|---------------|------------------|
| Distributed mutex (Redlock, etc.) | Complexity, partition pitfalls | [`race-condition-strategies.md`](race-condition-strategies.md) §1 note |
| OCC / CAS leases | Needs versioned store or schema | `tracks/02-optimistic-concurrency.md`, `04-atomic-cas.md` |
| Per-shard event loops | Needs sharding + refactor of server | `tracks/03-single-threaded-shard.md` |
| Append-only log | Different API than `Deque` | `tracks/05-append-only-log.md` |
| CRDTs | Conflicts with strict FIFO goal | `tracks/06-crds.md` |
| Multi-shard routing | Ops + partitioning design | `tracks/07-partitioning.md` |

---

## Sensible evolution paths (still in this repo’s universe)

Ordered by increasing scope — **docs and layout only** until you choose to build.

1. **`memory/ring.go`** — same mutex, better locality (not distributed).
2. **Blocking pop + `context`** — worker ergonomics, still one process.
3. **Graceful shutdown / health** on `cmd/queued` — ops hygiene.
4. **Partitioned brokers** — N deque instances or N servers with routing key (strategy §7).
5. **Lease table + OCC** beside HTTP pop — at-least-once semantics (strategies §2, §8).
6. **Separate “log track”** — new module, not a drop-in for `MemoryDeque` (strategy §5).

Hypothetical code layout for (4)–(6) is sketched in [`future-layout.md`](future-layout.md).

---

## Mental model

| Question | v1 answer |
|----------|-----------|
| Are we avoiding locks entirely? | **No** — we avoid *distributed* locks by **centralizing** the deque. |
| Is that how Redis works? | Redis **also** serializes commands per core/shard — different mechanism, same “one writer at a time” per shard. |
| When do we outgrow v1? | When one mutex + one server is too slow or you need replay / multi-region / leases. |

---

## Further reading in-repo

- [`race-condition-strategies.md`](race-condition-strategies.md) — full catalog
- [`design-chooser.md`](design-chooser.md) — pick a strategy for a new design
- [`tracks/README.md`](tracks/README.md) — optional deep dives
