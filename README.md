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

One Go `Deque` interface. Two planned implementations:

```
                    ┌─────────────────────┐
                    │   Deque interface   │
                    └──────────┬──────────┘
                               │
              ┌────────────────┴────────────────┐
              ▼                                 ▼
    ┌──────────────────┐              ┌──────────────────┐
    │   MemoryDeque    │              │   RemoteDeque    │
    │ (linked list +   │              │ (HTTP/RPC client)│
    │      mutex)      │              │                  │
    └──────────────────┘              └────────┬─────────┘
              │                                │
     one process, many goroutines      many processes ──► one queue server
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

### `RemoteDeque` (distributed — planned)

| | |
|---|---|
| **Storage** | A **single process** still owns the linked deque; other processes are clients |
| **Transport** | HTTP (or gRPC) — e.g. `POST /push`, `GET /pop` |
| **Best for** | Learning distribution without operating Redis; optional challenge track |

**Distributed** here means: many apps, **one logical deque**, state **not** in each client’s heap.

```
  Worker A ──┐
  Worker B ──┼──►  Queue server (owns MemoryDeque)  ◄── single source of truth
  API      ──┘
```

External stores (e.g. Redis) are **out of scope for v1**; you can add them later as another `RemoteDeque` backend once the local and HTTP paths are clear.

---

## Choosing an implementation

| Question | **MemoryDeque** | **RemoteDeque** |
|----------|-----------------|-----------------|
| Multiple OS processes? | No (unless you share memory externally) | Yes |
| Network round-trips? | No | Yes |
| Survives process restart? | No (unless you add persistence) | Only if the server persists |
| Complexity | Low | Medium (API, timeouts, errors) |
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

With **RemoteDeque**, the same rule applies at the server: concurrent client pops are serialized by the server’s mutex (or equivalent).

### Waiting when empty

| Approach | Behavior |
|----------|----------|
| Non-blocking pop | Return `ErrEmpty` immediately |
| Blocking pop (planned) | Wait on `sync.Cond` or channel until push or `ctx` cancel |
| Remote client | Long poll or server-side wait with timeout; map to `ErrTimeout` (planned) |

Do not spin-tight `Pop` in a loop when empty — block or sleep with backoff.

---

## Delivery semantics (honest defaults)

| Backend | After pop | If worker crashes mid-handler |
|---------|-----------|-------------------------------|
| **MemoryDeque** | Element removed from deque | Work is **lost** (at-most-once) unless you re-queue manually |
| **RemoteDeque** | Same, once server committed pop | Same; retries may need idempotent handlers |

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

## Suggested learning path

1. **`errors.go`** — `ErrEmpty`, `ErrClosed`.
2. **`deque.go`** — `Deque` interface.
3. **`memory/deque.go`** — doubly-linked list + mutex; table-driven unit tests.
4. **Concurrency test** — many goroutines push/pop one `MemoryDeque`.
5. **`cmd/queued` (optional)** — tiny HTTP server owning one deque; CLI workers call it.
6. **`remote/deque.go` (optional)** — `RemoteDeque` client wrapping that API.
7. **Integration test** — two processes or two HTTP clients, one server.

Deep dive: [`docs/deque-guide.md`](docs/deque-guide.md).

---

## Library roadmap

1. Typed errors (`ErrEmpty`, `ErrClosed`; later `ErrTimeout`, `ErrReadOnly`)
2. `context.Context` on all operations
3. `MemoryDeque` — full four ops + `Len` + `Close`
4. Blocking pop with `ctx` cancellation
5. Optional `RemoteDeque` + example queue server
6. Documented non-goals: exactly-once, built-in priority scheduler, Redis backend in v1
7. **(Later)** Ring-buffer `MemoryDeque` variant for allocation/cache tradeoffs

---

## Status

Early development. API details and examples land with `MemoryDeque` first.
