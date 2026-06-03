# Track 06 — Conflict-free replicated data types (CRDTs)

**Status:** Documentation only.

## Idea

Replicas apply updates **without** central locking; merge function guarantees all replicas converge to same state.

## Queue use case (limited)

- Queue **depth** counters, membership sets, presence — easier than strict global job order.
- Strict FIFO job delivery is **not** a natural CRDT fit.

## Race prevention

By construction for supported operations — “conflict-free” for that data type’s semantics.

## When to choose

- Partition tolerance + eventual consistency acceptable
- Ordering rules are commutative / last-write-wins with documented loss

## When to avoid

- Strict job ordering across regions
- “Exactly this job once globally in order”

## Examples

CRDT sets in collaborative systems; some metrics aggregation.

## This repo

Not planned for v1; documented for comparison when reading Redis/Kafka/CRDT literature.
