# Track 01 — Mutex + central owner (v1)

**Status:** Implemented in this repository.

## Idea

- One canonical deque in **one** process.
- `sync.Mutex` serializes push/pop/unlink.
- Other processes use HTTP (`cmd/queued`); they never hold the list.

## Race prevention

| Layer | Mechanism |
|-------|-----------|
| In-process | Mutex on `MemoryDeque` |
| Multi-process | Single server; mutex still on that deque |

## Files

- `memory/deque.go`, `memory/node.go`
- `cmd/queued/main.go`

## Exercises (no new code required)

1. Run `go test -race ./memory/...`
2. `go run ./cmd/queued` + parallel `curl` pops — each message once
3. Explain why two `queued` instances without routing = two queues (split brain)

## Limits

- Bottleneck: one server, one lock
- No durable replay; restart loses queue
- Pop + process crash = message lost (at-most-once)

## Next track

Compare with [`02-optimistic-concurrency.md`](02-optimistic-concurrency.md) for lease/claim without holding the deque lock during handler work.
