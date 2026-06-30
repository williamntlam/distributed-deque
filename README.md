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

### Avoiding race conditions without a distributed lock

v1 uses a **central owner + in-process mutex** — a deliberate first step, not the only pattern used in industry. For a full catalog (optimistic concurrency, event-loop shards, CAS, append-only logs, CRDTs, partitioning) and how each relates to this repo:

- **[`docs/distributed/README.md`](docs/distributed/README.md)** — index
- **[`docs/distributed/race-condition-strategies.md`](docs/distributed/race-condition-strategies.md)** — all strategies + summary table
- **[`docs/distributed/design-chooser.md`](docs/distributed/design-chooser.md)** — strict FIFO vs pub/sub / stream
- **[`docs/distributed/this-project.md`](docs/distributed/this-project.md)** — what v1 implements vs future tracks

| Strategy | When to use it | In this repo (v1) |
|----------|----------------|-------------------|
| Central mutex / single owner | Learning, one broker, strict FIFO | **Implemented** — [`tracks/01`](docs/distributed/tracks/01-mutex-central-owner.md), `memory/`, `cmd/queued` |
| Optimistic concurrency (OCC) | Rare per-row contention, versioned claims | Documented — [`tracks/02`](docs/distributed/tracks/02-optimistic-concurrency.md) |
| Single-threaded shard / event loop | High per-shard throughput | Documented — [`tracks/03`](docs/distributed/tracks/03-single-threaded-shard.md) |
| Atomic CAS | Fine-grained claims on external store | Documented — [`tracks/04`](docs/distributed/tracks/04-atomic-cas.md) |
| Append-only log | Streaming, replay, fan-out | Documented — [`tracks/05`](docs/distributed/tracks/05-append-only-log.md) |
| CRDTs | Eventual consistency, loose ordering | Documented — [`tracks/06`](docs/distributed/tracks/06-crds.md) |
| Partitioning | Scale past one mutex; order per key | Documented — [`tracks/07`](docs/distributed/tracks/07-partitioning.md) |

Distributed **mutexes** (Redlock, etc.) are discussed as a caution — not the default queue design. See the catalog for partition / split-brain notes.

### Coordination strategy lab — implement each track, then benchmark

Optional advanced path: build a **minimal prototype** for each [learning track](docs/distributed/tracks/README.md), run the **same workloads** where comparison is fair, and record throughput/latency so trade-offs are measurable — not only conceptual.

| # | Track | Status | Experiment sketch | What to time |
|---|--------|--------|-------------------|--------------|
| 01 | [Mutex + central owner](docs/distributed/tracks/01-mutex-central-owner.md) | **Implemented** | `memory/deque.go`, `cmd/queued` — baseline | `go test -bench` on `MemoryDeque`; HTTP load on `queued` |
| 02 | [Optimistic concurrency](docs/distributed/tracks/02-optimistic-concurrency.md) | Planned | Versioned lease table beside deque; claim without holding deque lock during handler work | Claim ops/sec; **retry rate** when workers collide |
| 03 | [Single-threaded shard](docs/distributed/tracks/03-single-threaded-shard.md) | Planned | Channel of ops → **one goroutine** owns deque per shard (same FIFO semantics, no mutex on nodes) | Per-shard throughput vs Track 01 mutex |
| 04 | [Atomic CAS](docs/distributed/tracks/04-atomic-cas.md) | Planned | Conditional claim (`WHERE status='AVAILABLE'` or in-memory CAS for tests) | CAS success vs failure under N workers |
| 05 | [Append-only log](docs/distributed/tracks/05-append-only-log.md) | Planned | Separate module: append records; consumers advance offset (see [`future-layout.md`](docs/distributed/future-layout.md)) | Append ingest rate; offset commit contention |
| 06 | [CRDTs](docs/distributed/tracks/06-crds.md) | Planned | CRDT counter/set **beside** queue — not strict global FIFO | Merge/converge time; compare on supported ops only |
| 07 | [Partitioning](docs/distributed/tracks/07-partitioning.md) | Planned | `hash(key) % N` → N deques or N `queued` instances with routing | Aggregate throughput; **hot partition** behavior |

**Suggested build order:** 01 (baseline numbers first) → 02–04 (coordination styles) → 05–06 (different data models) → 07 (scale). Finish one track, add benchmarks, save results, then move on.

**Benchmark scenarios** (reuse across tracks that hand work to workers):

| Scenario | Purpose |
|----------|---------|
| **Independent jobs** | Many workers, different messages/rows — low contention (OCC/CAS should win on claim latency) |
| **Hot key / hot shard** | All workers fight one row or partition — measure retries and tail latency |
| **Strict FIFO (Mode A)** | One producer, one consumer — correctness baseline, not max throughput |
| **Multi-process** | `cmd/queued` or per-shard servers + parallel HTTP clients (`curl`, `hey`, `wrk`) |

**Metrics to record:** throughput (ops/sec or jobs/sec), p50/p99 latency, failed claims / retries (OCC & CAS), and hardware notes (`GOMAXPROCS`, machine). Keep a simple results log (e.g. `docs/distributed/benchmarks.md` when you have numbers).

**Fair comparison notes:**

- **CRDTs** and **append-only logs** target different problems than an in-memory deque — benchmark the workload they actually solve (metrics merge, replay/stream), not only `PopFront` nanoseconds.
- **Partitioning** raises total throughput by adding shards; it does not preserve global order across keys.
- HTTP-based tracks include network overhead that in-process `go test -bench` skips — label results accordingly.

Hypothetical package layout per track: [`docs/distributed/future-layout.md`](docs/distributed/future-layout.md).

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
    ├── deque-guide.md
    └── distributed/              # design docs only (no implementation)
        ├── README.md
        ├── race-condition-strategies.md
        ├── this-project.md
        ├── design-chooser.md
        ├── future-layout.md      # hypothetical code paths
        └── tracks/               # per-strategy outlines (01–07)
            ├── README.md
            ├── 01-mutex-central-owner.md
            ├── 02-optimistic-concurrency.md
            ├── 03-single-threaded-shard.md
            ├── 04-atomic-cas.md
            ├── 05-append-only-log.md
            ├── 06-crds.md
            └── 07-partitioning.md
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
8. **(Optional) Coordination strategy lab** — implement tracks [02–07](docs/distributed/tracks/README.md) one at a time; benchmark each against Track 01 baseline (see [Coordination strategy lab](#coordination-strategy-lab--implement-each-track-then-benchmark)).

Deep dive: [`docs/deque-guide.md`](docs/deque-guide.md). Distributed strategies: [`docs/distributed/README.md`](docs/distributed/README.md).

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
9. **(Later)** Coordination strategy lab — minimal implementation + benchmarks per [track](docs/distributed/tracks/README.md) (02–07)

---

## Status

Early development. API details and examples land with `MemoryDeque` first.
