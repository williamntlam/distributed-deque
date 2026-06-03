# Track 07 — Partitioning (sharding)

**Status:** Documentation only (README “Mode C” points here conceptually).

## Idea

Route each key to an independent queue instance so workers only contend **within** a partition.

```text
tenant-A → deque A → consumer pool A
tenant-B → deque B → consumer pool B
```

## Race prevention

Smaller critical sections; no cross-partition lock.

## Relation to README Mode C

Many producers → **one broker per partition key** → one consumer per partition for strict order **within** key.

## Hypothetical repo touchpoints

- Multiple `cmd/queued` processes with routing table
- Or multiple `MemoryDeque` inside one server keyed by string

See [`../future-layout.md`](../future-layout.md).

## Study questions

1. Hot partition problem?
2. How is this different from Kafka partitions?
3. Two uncoordinated `queued` on same key — split brain?

## When to choose

- Scale past one mutex
- Order only per `user_id` / `shard_id`

## v1

Single global deque — simplest learning path.
