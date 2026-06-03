# Track 05 — Append-only immutable log

**Status:** Documentation only.

## Idea

Producers **append** records; consumers track **offset**. Log segments are not updated in place for “delete.”

## Race prevention

Readers do not fight over mutating the same cell; offset commit may use OCC/CAS.

## vs this repo’s `Deque`

| Deque (v1) | Log |
|------------|-----|
| Pop removes node | Consume advances offset |
| O(1) both ends (API) | Append fast; trim/compaction policy |
| One in-memory structure | Durable segments, partitions |

## When to choose

- Replay, fan-out consumer groups, very high ingest
- Pub/sub and stream processing mental model

## Examples

Kafka, Pulsar, NATS JetStream (log-like).

## If you built it here

Would likely be a **separate module** (`log/` or similar) — see [`../future-layout.md`](../future-layout.md). Not a rename of `memory/deque.go`.

## Study questions

1. Where does FIFO live in a partitioned log?
2. How do consumer offsets race?
