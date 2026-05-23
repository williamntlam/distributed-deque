# Deque guide for distributed-deque

A learning-oriented reference for the **in-memory doubly-linked list** backend and how to grow it into a **distributed** deque. Pair with [`README.md`](../README.md) (design and **Repository layout**) and [`.cursor/rules/memory-patterns.mdc`](../.cursor/rules/memory-patterns.mdc) (agent hints).

**Where code lives:** `memory/node.go` (nodes), `memory/deque.go` (`MemoryDeque`), tests in `memory/deque_test.go` — not under `list/` or `stream/`.

---

## 1. Why a doubly-linked list (not a slice)?

A deque needs **O(1)** insert and remove at **both** ends:

| Structure | `PushFront` / `PopFront` | `PushBack` / `PopBack` |
|-----------|--------------------------|-------------------------|
| **Slice** | **O(n)** — prepend shifts all elements | **O(1)** amortized append |
| **Doubly-linked list** | **O(1)** | **O(1)** |

So `MemoryDeque` uses a **doubly-linked** list: each node has `prev`, `next`, and a `[]byte` payload. Keep `head`, `tail`, and a `size` field for **O(1)** `Len`.

In one Go process, **linked list + mutex** is enough:

```
  goroutine A ──┐
  goroutine B ──┼──►  MemoryDeque { mu, head, tail, size }  ◄── one heap, one owner
  goroutine C ──┘
```

Implementation options:

- **`container/list`** — stdlib doubly-linked list; learn the API, wrap with mutex.
- **Custom `node` struct** — more explicit; good for learning pointers.

Properties you get:

- **Fast at both ends** — true deque semantics without shifting a slice.
- **Clear semantics** — you own lock boundaries and when a node is unlinked.
- **Real concurrency practice** — same API you will expose over HTTP later.

Tradeoffs vs a slice: one allocation per element, worse CPU cache locality. For learning and typical task payloads, that is acceptable.

### Ring buffer (later optimization)

After the linked-list deque works and has tests, you can swap the backing store for a **ring buffer** (fixed or growable `[]byte` slots + head/tail indices modulo capacity):

| | Doubly-linked list | Ring buffer |
|---|-------------------|-------------|
| Push/pop both ends | O(1) | O(1) |
| Allocations | Per node | Amortized fewer (reuse slots) |
| Cache locality | Weaker | Stronger |
| Max size | Heap-limited | Fixed cap unless you grow the buffer |

Same public `Deque` API; only `memory/` internals change. Good exercise once pointer juggling is comfortable.

What a local deque does **not** give you: sharing across processes, survival after exit, or HA — that is the next lesson.

---

## 2. Picture the in-memory deque

Logical order (links, not indices):

```
HEAD (front)                                              TAIL (back)
   │                                                          │
   ▼                                                          ▼
 nil ◄──► [ urgent ] ◄──► [ task-B ] ◄──► [ task-A ] ◄──► nil
              ▲                              ▲
       PushFront / PopFront            PushBack / PopBack
```

| Your API | Implementation sketch |
|----------|------------------------|
| `PushFront` | Lock; new node; link before `head`; update `head`; `size++` |
| `PushBack` | Lock; new node; link after `tail`; update `tail`; `size++` |
| `PopFront` | Lock; unlink `head`, advance `head`; `size--`; empty → `ErrEmpty` |
| `PopBack` | Lock; unlink `tail`, retreat `tail`; `size--` |
| `Len` | Lock; return `size` (do not walk the list each time) |

Empty deque: `head == nil`, `tail == nil`, `size == 0`.

---

## 3. Concurrency: many goroutines, one deque

Ten goroutines calling `PopFront` on the **same** `MemoryDeque`:

- The mutex ensures **one pop at a time**; each call gets a **different** element or `ErrEmpty`.
- You do **not** need a second mutex in the caller.
- Order between goroutines is **who acquires the lock first**, not fair scheduling across priorities.

| Situation | Result |
|-----------|--------|
| Enough items | Each pop removes one distinct element |
| One item, two pops | First succeeds; second `ErrEmpty` |
| Empty deque | `ErrEmpty` (non-blocking) |

This mirrors what Redis `LPOP` gives you at the server — but here **you** are the server and the lock is in Go.

---

## 4. Blocking vs empty (local)

| Mode | Behavior when empty |
|------|---------------------|
| Non-blocking `PopFront` | Return `ErrEmpty` immediately |
| Blocking pop (planned) | `sync.Cond` or channel: wait until push or `ctx.Done()` |

**Best practice:** Worker loops use blocking pop with `context` cancel on shutdown, not `for { Pop(); sleep }`.

### Sync now, async later (planned)

The public `Deque` interface stays **synchronous** in v1: `PopFront` / `PopBack` return when the call finishes (or `ErrEmpty`). The mutex is only for safe, short critical sections — not a substitute for “wait until work arrives.”

| Layer | v1 | Later (optional) |
|-------|----|------------------|
| **MemoryDeque** | Non-blocking pop → `ErrEmpty` | Blocking pop (`sync.Cond` + `ctx`); async helpers (e.g. result channel) |
| **App / RemoteDeque** | Caller may use `go func() { d.PopFront(ctx) }()` | Dedicated `*Async` helpers; HTTP long-poll where relevant |

Async features will sit **above** sync push/pop — same `MemoryDeque` internals, extra ergonomics for workers and remote clients. See README [Sync API now, async later](../README.md#sync-api-now-async-later-planned).

---

## 5. Making it distributed

**Distributed** = multiple **processes** (or machines) share **one logical deque**.

The linked deque inside Worker A is invisible to Worker B. You need a **single owner**:

```
  Worker A ──┐
  Worker B ──┼──►  queue-server process (owns MemoryDeque)
  API      ──┘
```

### 5.1 Queue-server pattern (recommended challenge)

One binary holds the only `MemoryDeque`. Others use HTTP (or gRPC):

| HTTP (example) | Maps to |
|----------------|---------|
| `POST /push/back` body | `PushBack` |
| `GET /pop/front` | `PopFront` → 200 + body or 204 empty |

You learn:

- Network ≠ mutex — timeouts, connection errors, ambiguous failures
- **Empty** → 204 or typed JSON, not confused with “server down”
- One pop per request still atomic **on the server** because one mutex protects the linked deque

### 5.2 What still breaks (same as any queue)

| Scenario | Behavior |
|----------|----------|
| Pop succeeded, worker crashes before work | Message **lost** unless you add ack/retry design |
| Client timeout, unsure if pop ran | Do not blindly assume; use idempotency |
| Two queue servers, no coordination | **Split brain** — two different deques; avoid for v1 |

### 5.3 Limitations (any distributed deque)

| Limitation | Angle |
|------------|--------|
| **Exactly-once** | Pop removes from shared state; crash after pop = at-most-once unless you add ack ledger |
| **Priorities** | Not automatic; use separate deques or server-side policy |
| **Multi-step workflows** | Pop + DB write is two steps — not one atomic transaction |

---

## 6. Local vs remote comparison

| Concern | MemoryDeque | RemoteDeque (HTTP server) |
|---------|-------------|---------------------------|
| Latency | Lowest | Network RTT |
| Multi-process | No | Yes (via server) |
| Restart | Deque gone | Gone unless server persists |
| Empty signal | `ErrEmpty` | Map from HTTP 204 / body |
| Infra errors | N/A | Distinct from `ErrEmpty` |

---

## 7. Failures you should understand

| Scenario | MemoryDeque | RemoteDeque |
|----------|-------------|-------------|
| Pop then crash before handler | Item lost | Item lost (server already popped) |
| `Close()` on deque | `ErrClosed` on ops | Client should stop; server may drain |
| Many concurrent pops | Safe (mutex) | Safe if server uses one MemoryDeque |

Always separate **`ErrEmpty`**, **`ErrClosed`**, and **infrastructure errors**.

---

## 8. Go mapping (when you code)

Node + deque shape (illustrative):

```go
type node struct {
    value []byte
    prev  *node
    next  *node
}

type Deque struct {
    mu    sync.Mutex
    head  *node
    tail  *node
    size  int
    closed bool
}
```

`PopFront` sketch:

```go
func (d *Deque) PopFront(ctx context.Context) ([]byte, error) {
    d.mu.Lock()
    defer d.mu.Unlock()
    if d.closed {
        return nil, distributeddeque.ErrClosed
    }
    if d.head == nil {
        return nil, distributeddeque.ErrEmpty
    }
    n := d.head
    d.head = n.next
    if d.head != nil {
        d.head.prev = nil
    } else {
        d.tail = nil // last element
    }
    d.size--
    return n.value, nil // or copy: append([]byte(nil), n.value...)
}
```

- Check **`ctx.Err()`** before blocking waits (when you add them).
- Do not return `ErrEmpty` for canceled context — return `ctx.Err()`.
- **Copy** `[]byte` on pop if callers may retain the slice after the node is freed for GC.

---

## 9. Local practice

No Docker required for v1:

```bash
go test ./...
go test -race ./memory/...
```

Optional: run a queue server and two worker CLIs in separate terminals once `cmd/queued` exists.

---

## 10. Suggested learning path

1. **`errors.go`** — `ErrEmpty`, `ErrClosed`.
2. **`deque.go`** — interface.
3. **`memory/node.go`** — node + link helpers.
4. **`memory/deque.go`** + **`memory/deque_test.go`** — four ops, `size`, mutex.
5. **Race test** — `go test -race ./memory/...`.
6. **`cmd/queued`** — queue server (one owner).
7. **`remote/deque.go`** — HTTP client.
8. **`test/integration/remote_deque_test.go`** — multi-client.
9. **`memory/ring.go`** — ring-buffer optimization.
10. **Async helpers / blocking pop** — after sync API and tests are solid.

Update [`AGENTS.md`](../AGENTS.md) when you complete each milestone.
