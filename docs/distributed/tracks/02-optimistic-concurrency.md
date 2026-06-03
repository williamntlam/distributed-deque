# Track 02 — Optimistic concurrency control (OCC)

**Status:** Documentation only.

## Idea

Allow concurrent reads; commit only if `version` (or timestamp) still matches. On conflict, retry or skip.

## Queue use case

- **Lease table** next to the queue: claim row `version=1` → `version=2` when one worker wins.
- Deque body can stay mutex-backed; OCC on **metadata** is enough for many designs.

## Race prevention

Second writer fails commit when version advanced — no long-lived distributed lock.

## Hypothetical repo touchpoints

See [`../future-layout.md`](../future-layout.md) — `internal/lease/` (not created).

## Study questions

1. How is OCC different from “lock row before read”?
2. What happens under high contention on one hot message?
3. How does SQS visibility timeout relate to lease + version?

## When to choose

- Rare conflicts, versioned store available
- Workers need time to process without blocking other **claims** on different messages

## Not chosen in v1 because

No versioned store; learning focus is deque + HTTP first.
