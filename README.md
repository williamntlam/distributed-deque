# distributed-deque

A Go library for a **double-ended queue (deque)** with a path from **in-process** storage to **distributed** coordination. You start with an in-memory **doubly-linked list** (`[]byte` payloads per node, `sync.Mutex`) so push/pop at both ends are **O(1)**, then extend to multiple processes by giving **one owner** of that deque and talking to it over the network.

The name **distributed-deque** describes the learning goal: understand what changes when deque state must be shared across workers, not only across goroutines.

---

## Repository naming

| | |
|---|---|
| **Initial proposal** | `distributed-queue` |
| **Final name** | `distributed-deque` |

A standard **queue** implies strict FIFO. This project supports **bidirectional** push/pop at both ends (head and tail). **Deque** matches that behavior.

---

## What is a deque?

| Operation | Aliases | Description |
|-----------|---------|-------------|
| `PushFront` | Prepend | Insert at the **head** |
| `PushBack` | Append | Insert at the **tail** |
| `PopFront` | Shift | Remove from the **head** |
| `PopBack` | Pop | Remove from the **tail** |

---

## Architecture

One Go `Deque` interface. **In-process:** `MemoryDeque`. **Distributed:** `cmd/queued` owns the only deque and exposes HTTP; other processes use **any HTTP client** (e.g. `curl`) — there is no `remote` Go package in this repo.

```
                    ┌─────────────────────┐
                    │   Deque interface   │
                    └──────────┬──────────┘
                               │
              ┌────────────────┴────────────────┐
              ▼                                 ▼
    ┌──────────────────┐              ┌──────────────────┐
    │   MemoryDeque    │              │  cmd/queued      │
    │ (linked list +   │              │  HTTP POST /push │
    │      mutex)      │              │  GET /pop        │
    └──────────────────┘              └────────┬─────────┘
              │                                │
     one process, many goroutines      curl / scripts / apps ──► one queue server
```

### `MemoryDeque` (in-memory doubly-linked list)

| | |
|---|---|
| **Storage** | Doubly-linked nodes, each holding a `[]byte` payload; `head` / `tail` pointers + `size` counter |
| **Ops** | `PushFront`, `PushBack`, `PopFront`, `PopBack`, `Len` — **O(1)** at both ends (with a maintained `size`) |
| **Concurrency** | `sync.Mutex` on the struct (hold for whole push/pop; unlink is not safe unlocked) |
| **Best for** | Single binary, worker pools, unit tests, learning deque semantics |

Use a **doubly-linked** list, not a Go slice: prepending at the front of a slice is **O(n)** because elements shift. A linked list matches true deque semantics at both ends.

**Later optimization:** a **ring buffer** (circular slice with head/tail indices) also gives **O(1)** push/pop at both ends with better cache locality and fewer per-element allocations — consider it once the linked-list version is correct and tested.

Many goroutines can call `PopFront` concurrently; the mutex serializes access so each pop removes **one** distinct element (or returns `ErrEmpty`).

### `cmd/queued` (distributed — HTTP server)

| | |
|---|---|
| **Storage** | A **single process** owns the linked deque (`MemoryDeque`) |
| **Transport** | HTTP — `POST /push` → `PushBack`, `GET /pop` → `PopFront` (204 if empty) |
| **Clients** | `curl`, scripts, or your own apps — not a bundled Go HTTP client package |

**Distributed** here means: many apps, **one logical deque**, state **not** in each client’s heap.

```
  Worker A (curl) ──┐
  Worker B (curl) ──┼──►  cmd/queued (owns MemoryDeque)  ◄── single source of truth
  API (curl)      ──┘
```

External stores (e.g. Redis) are **out of scope for v1**.

---

## Choosing an implementation

| Question | **MemoryDeque** | **`cmd/queued` + HTTP** |
|----------|-----------------|-------------------------|
| Multiple OS processes? | No (unless you share memory externally) | Yes |
| Network round-trips? | No | Yes |
| Survives process restart? | No (unless you add persistence) | Only if the server persists |
| Complexity | Low | Medium (HTTP contract, status codes) |
| Good first step? | **Yes** | After `MemoryDeque` works |

---

## Distributed coordination (conceptual)

| Scope | Who shares the deque? | Mechanism |
|-------|----------------------|-----------|
| **In-process** | Goroutines in one binary | Mutex around the linked deque |
| **Multi-process** | Separate programs | One owner + RPC; clients never hold the canonical list |
| **Multi-machine** | Same as multi-process | Deploy the queue server reachable on the network |

What you **do not** get for free: exactly-once processing, fair priority scheduling across tenants, or atomic “pop + update two queues” unless you design it.

### Many workers popping

With **one** `MemoryDeque` behind the server:

- Each successful `PopFront` removes one element under the mutex — no duplicate element from two pops.
- With **separate** `MemoryDeque` instances per process, each has its own list — that is **not** distributed; workers do not share work.

With **`cmd/queued`**, the same rule applies: concurrent HTTP pops are serialized by the server’s mutex.

### Ordering: strict FIFO vs concurrent workers

`MemoryDeque` is **thread-safe** for both patterns below. **Order policy is your choice** — the library gives FIFO **at the head** on each successful pop, not global business workflow rules.

#### Mode A — Strict / reproducible order

Use when you need the **same append and processing sequence every run** (or a single global timeline).

```text
one producer  ──►  PushBack…  ──►  one MemoryDeque  ──►  PopFront…  ──►  one consumer
   (goroutine)                                              (goroutine)
```

| Piece | Rule |
|-------|------|
| **Producers** | **One** goroutine pushing in sequence (simplest), *or* many writers through **one broker** where you accept **arrival order** (may differ run-to-run) |
| **Deque ops** | Pick one lane — e.g. **`PushBack` + `PopFront`** (queue FIFO) |
| **Consumers** | **One** goroutine popping |
| **Tests in repo** | `TestPushBack_PopFront_FIFO`, `TestPushFront_PopBack_LIFOAtBack`, other single-goroutine tests |

What you get: deterministic tests; processing matches queue order front → back.

What you do **not** get automatically: a fixed business checklist (“step 2 always runs”) — only FIFO for events **actually enqueued**.

#### Mode B — Concurrent worker pool

Use when tasks are **independent** and you want **throughput + mutex safety**, not a fixed global processing timeline.

```text
many producers ──►  PushBack…  ──►  one MemoryDeque  ◄──  PopFront…  ── many consumers
                                         ▲
                                    mutex serializes
                                    each push/pop
```

| Piece | Rule |
|-------|------|
| **Producers / consumers** | **Many** goroutines (or many HTTP clients to `cmd/queued`) |
| **Safety** | Mutex / queue server prevents corruption; `-race` should pass |
| **Order** | **Non-deterministic** interleaving — snapshot of the list and **who** popped what changes run-to-run |
| **Tests in repo** | `TestConcurrentPushPop` |

What you get: safe sharing; each successful pop still removes the **current head** (FIFO at that instant).

What you do **not** get: same enqueue order every run; strict global **processing** order across workers.

#### Mode C — Many writers, one broker, one consumer (distributed strict processing)

Strict **processing** order without a single producer thread:

```text
many producers  ──►  cmd/queued / broker (mutex)  ──►  one deque  ──►  one consumer
```

Append order = **broker timeline** (stable **within** a run; may vary **across** runs if producers race). Processing order = FIFO if **one** popper.

#### Quick chooser

| Need | Mode |
|------|------|
| Exact test assertions every run | **A** — single goroutine in test, or one producer + one consumer |
| Parallel workers, order doesn’t matter | **B** — `TestConcurrentPushPop` style |
| Many API servers, one ordered pipeline | **C** — broker + **one** consumer |
| Order per user / tenant | **Partition** — `deque:{id}` per entity + one consumer **per partition** |

See [`docs/deque-guide.md`](docs/deque-guide.md) §3 (concurrency) and §5 (broker) for detail.

### Waiting when empty

| Approach | Behavior |
|----------|----------|
| Non-blocking pop | Return `ErrEmpty` immediately |
| Blocking pop (planned) | Wait on `sync.Cond` or channel until push or `ctx` cancel |
| Remote client | Long poll or server-side wait with timeout; map to `ErrTimeout` (planned) |

Do not spin-tight `Pop` in a loop when empty — block or sleep with backoff.

### Sync API now, async later (planned)

**v1** keeps the public [`Deque`](deque.go) interface **synchronous**: each call returns when the operation finishes (or returns `ErrEmpty`). A `sync.Mutex` on `MemoryDeque` only serializes short pointer updates — it is not the same as “wait until a message arrives” or blocking on the network.

| Concern | v1 | Planned later |
|---------|----|----------------|
| Empty pop | `ErrEmpty` immediately | Optional **blocking pop** (`sync.Cond` + `context`) for workers |
| Caller blocking | Caller goroutine waits for the op | Optional **async helpers** (e.g. `PopFrontAsync` → `<-chan` result, or app wraps sync calls in `go func() { ... }()`) |
| HTTP clients (`curl`, etc.) | Manual requests | Async / long-poll patterns optional later |

Async will be added **on top of** the sync implementation — not by removing the mutex. Typical pattern: worker runs sync `PopFront` in a background goroutine, or a thin helper returns a channel that receives `{value, err}` when done.

---

## Delivery semantics (honest defaults)

| Backend | After pop | If worker crashes mid-handler |
|---------|-----------|-------------------------------|
| **MemoryDeque** | Element removed from deque | Work is **lost** (at-most-once) unless you re-queue manually |
| **`cmd/queued`** | Same, once server committed pop | Same; retries may need idempotent handlers |

**Exactly-once** is not a goal of v1. Use idempotency keys in the application if retries happen.

---

## Production considerations (lightweight)

Focused on what matters for the in-memory / queue-server track:

| Topic | Guidance |
|-------|----------|
| **Errors** | `ErrEmpty` vs `ErrClosed` vs network/HTTP failures — never conflate |
| **Context** | Pass `context.Context`; cancel blocking waits on shutdown |
| **Close** | After `Close()`, reject ops with `ErrClosed` |
| **Payload size** | Keep `[]byte` bounded; large blobs belong in object storage with a pointer in the deque |
| **Graceful shutdown** | Stop accepting pushes; drain or cancel workers; document in-flight items |
| **Observability** | Log depth, pop/push rates, handler errors (planned helpers) |

---

## Repository layout

Target tree for v1 (doubly-linked `MemoryDeque`). **No** `list/`, `stream/`, or Redis client packages.

```
distributed-deque/
├── README.md
├── AGENTS.md
├── go.mod
├── go.sum
│
├── errors.go                 # distributeddeque — ErrEmpty, ErrClosed
├── deque.go                  # distributeddeque — Deque interface
│
├── memory/                   # package memory — in-process deque
│   ├── node.go               # node { value, prev, next } and link/unlink helpers
│   ├── deque.go              # MemoryDeque { mu, head, tail, size, closed }
│   ├── deque_test.go         # Mode A (FIFO) + Mode B (TestConcurrentPushPop) tests
│   └── ring.go               # (later) optional ring-buffer backing, same API
│
├── cmd/
│   └── queued/               # queue server binary
│       └── main.go           # owns the only MemoryDeque; HTTP push/pop API
│
└── docs/
    └── deque-guide.md
```

| Path | Package | Role |
|------|---------|------|
| Root `.go` files | `distributeddeque` | Public contract: interface, errors; importers use `memory.NewMemoryDeque()` |
| `memory/node.go` | `memory` | Doubly-linked **node** type; keeps pointer logic separate from `MemoryDeque` |
| `memory/deque.go` | `memory` | **O(1)** push/pop at head/tail; returns root `ErrEmpty` / `ErrClosed` |
| `memory/ring.go` | `memory` | **Later** — swap internals only; same `MemoryDeque` methods |
| `cmd/queued/` | `main` | Single owner of canonical deque; HTTP API for other processes |

**Tests:** `memory/deque_test.go`. Exercise distribution with `go run ./cmd/queued` and `curl` (see server section).

---

## Suggested learning path

1. **`errors.go`** — `ErrEmpty`, `ErrClosed`.
2. **`deque.go`** — `Deque` interface.
3. **`memory/node.go`** — node struct and link helpers.
4. **`memory/deque.go`** + **`memory/deque_test.go`** — deque + **Mode A** tests (strict FIFO, single goroutine).
5. **Mode B concurrency test** — `TestConcurrentPushPop`; `go test -race ./memory/...`.
6. **`cmd/queued`** — queue server owning one deque; test with `curl`.
7. **`memory/ring.go` (later)** — ring-buffer optimization.

Deep dive: [`docs/deque-guide.md`](docs/deque-guide.md).

---

## Library roadmap

1. Typed errors (`ErrEmpty`, `ErrClosed`; later `ErrTimeout`, `ErrReadOnly`)
2. `context.Context` on all operations
3. `MemoryDeque` — full four ops + `Len` + `Close`
4. Blocking pop with `ctx` cancellation
5. `cmd/queued` queue server + HTTP (`POST /push`, `GET /pop`); clients via `curl` or any HTTP tool
6. Documented non-goals: exactly-once, built-in priority scheduler, Redis backend in v1
7. **(Later)** Ring-buffer `MemoryDeque` variant for allocation/cache tradeoffs
8. **(Later)** Optional async client helpers and blocking pop (see [Sync API now, async later](#sync-api-now-async-later-planned))

---

## Status

Early development. API details and examples land with `MemoryDeque` first.
