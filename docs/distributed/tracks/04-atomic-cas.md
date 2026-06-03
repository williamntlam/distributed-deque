# Track 04 — Atomic compare-and-swap (CAS)

**Status:** Documentation only.

## Idea

Storage executes “set new value **only if** current equals expected” as one atomic step.

## Queue use case

```text
UPDATE messages SET status='PROCESSING'
  WHERE id=? AND status='AVAILABLE'
```

One row affected = you won the claim.

## Race prevention

No read-modify-write race in application code — store enforces atomicity.

## Hypothetical repo touchpoints

`internal/store/` adapter — see [`../future-layout.md`](../future-layout.md).

## Study questions

1. CAS vs mutex — when is CAS enough?
2. What failure modes remain after a successful CAS claim? (worker crash)

## When to choose

- etcd/Consul/SQL as source of truth for message state
- Fine-grained claims per message

## Not in v1

In-memory linked list without external atomic store.
