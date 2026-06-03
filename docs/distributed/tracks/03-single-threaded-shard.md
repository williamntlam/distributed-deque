# Track 03 — Single-threaded event loop per shard

**Status:** Documentation only.

## Idea

All mutations for shard *K* run on **one** thread/goroutine sequentially — no lock on structures if only that thread touches them.

## Queue use case

- Shard by `hash(job_key) % N`
- 10k concurrent HTTP pops to shard 3 → serialized at handler queue

## Race prevention

Concurrency becomes **ordering of requests**, not lock contention on shared nodes.

## Relation to this repo

`cmd/queued` today: many goroutines + **one mutex**. Evolution: channel of ops → single worker goroutine owning deque (same semantics, different style).

## Study questions

1. How is Redis’s single-threaded core similar/different?
2. What breaks if two processes both “own” shard 3?

## When to choose

- Extreme per-shard throughput
- Willing to partition explicitly

## Examples

Redis per instance; actor model per partition.
