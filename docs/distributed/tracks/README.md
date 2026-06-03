# Learning tracks — distributed queue strategies

Each file is an **outline** for reading and future experiments. **No Go code** is required to complete a track on paper.

| # | Track | Status in repo |
|---|--------|----------------|
| 01 | [Mutex + central owner](01-mutex-central-owner.md) | **Implemented** — `memory/`, `cmd/queued` |
| 02 | [Optimistic concurrency](02-optimistic-concurrency.md) | Documented only |
| 03 | [Single-threaded shard / event loop](03-single-threaded-shard.md) | Documented only |
| 04 | [Atomic CAS](04-atomic-cas.md) | Documented only |
| 05 | [Append-only log](05-append-only-log.md) | Documented only |
| 06 | [CRDTs](06-crds.md) | Documented only |
| 07 | [Partitioning](07-partitioning.md) | Documented only |

Suggested order: **01** (what you built) → read **02–04** (coordination) → **05–06** (different data models) → **07** (scale).

Parent catalog: [`../race-condition-strategies.md`](../race-condition-strategies.md).
