# Redis guide for distributed-deque

A learning-oriented reference for how Redis backs this library. Pair with [`README.md`](../README.md) (design) and [`.cursor/rules/redis-patterns.mdc`](../.cursor/rules/redis-patterns.mdc) (agent hints).

---

## 1. Why Redis for a distributed deque?

Your Go process memory is **local**. Other workers cannot see it. Redis holds the **shared state** so:

```
  Worker A ──┐
  Worker B ──┼──►  Redis key "deque:tasks"  ◄── single source of truth
  API      ──┘
```

Properties you get:

- **Atomic commands** — `LPOP` and `LPUSH` run atomically; no lost updates from two clients racing.
- **Low latency** — in-memory data structure with optional persistence.
- **Familiar ops** — lists and streams map reasonably to queue/deque patterns.

What Redis does **not** give you for free: exactly-once delivery, fair scheduling across priorities, or cross-key transactions without careful design.

---

## 2. Core Redis data structures (relevant here)

| Type | Mental model | Used by |
|------|----------------|---------|
| **List** | Doubly-linked list of strings | `ListDeque` |
| **Stream** | Append-only log with IDs | `StreamDeque` |

Other types (String, Hash, Set, Sorted Set) are not the primary deque store in this project.

---

## 3. ListDeque — Redis Lists

### 3.1 Picture the list

Redis list for key `deque:jobs`:

```
HEAD (left)                         TAIL (right)
   │                                    │
   ▼                                    ▼
 [ task-C ] <-> [ task-B ] <-> [ task-A ]
```

| Your API | Command | Effect |
|----------|---------|--------|
| `PushFront` | `LPUSH deque:jobs "payload"` | Insert at head (left) |
| `PushBack` | `RPUSH deque:jobs "payload"` | Insert at tail (right) |
| `PopFront` | `LPOP deque:jobs` | Remove from head |
| `PopBack` | `RPOP deque:jobs` | Remove from tail |
| `Len` | `LLEN deque:jobs` | Count elements |

Try in `redis-cli`:

```bash
DEL deque:jobs
RPUSH deque:jobs "first"
RPUSH deque:jobs "second"
LPUSH deque:jobs "urgent"
LRANGE deque:jobs 0 -1    # see order without removing
LPOP deque:jobs
```

### 3.2 Blocking vs empty

| Mode | Behavior when empty |
|------|---------------------|
| `LPOP` | Returns nil → map to **`ErrEmpty`** in Go |
| `BLPOP key timeout` | Blocks until item or timeout → map timeout to **`ErrTimeout`** (planned) or `context.DeadlineExceeded` |

**Best practice:** Workers should use **blocking** pops with a timeout tied to `context.Context`, not tight loops that hammer Redis.

### 3.3 List mode tradeoffs

| Pros | Cons |
|------|------|
| Simple, fast | Pop **destroys** the message — no replay |
| Easy deque at both ends | No built-in “processing” state |
| Small memory footprint | Crash after pop but before work = **lost** message |

Good for: task routing, work queues where loss on crash is acceptable or handled elsewhere.

### 3.4 Many consumers

Ten workers all calling `LPOP` on the same key:

- Each pop gets **one** element.
- Redis guarantees no duplicate delivery **for that pop**.
- Order is **per-command**, not globally “fair” across workers.

---

## 4. StreamDeque — Redis Streams

### 4.1 Picture the stream

A stream is an **append-only log** of entries, each with an ID (timestamp-based):

```
STREAM deque:events
  1740000000000-0  field message "a"
  1740000000001-0  field message "b"
  1740000000002-0  field message "c"
```

`XADD` appends. Entries stay until trimmed or deleted by policy.

### 4.2 Consumer groups (why they exist)

Without groups, every reader sees the full stream. **Consumer groups** assign messages to consumers and track **pending** (delivered but not acked):

```
XADD → entries in stream
XREADGROUP → deliver to consumer "worker-1"
             (entry moves to PEL = pending list)
handler runs...
XACK → remove from PEL (done)
```

| Concept | Meaning |
|---------|---------|
| **PEL** | Pending Entry List — in-flight work |
| **XACK** | “I finished this ID” |
| **XCLAIM / XAUTOCLAIM** | Another worker takes stale pending messages |

**Best practice:** **XACK only after** your side effects are durable (DB commit, etc.). Ack-first loses at-least-once safety.

### 4.3 Delivery guarantees

| Mode | Guarantee |
|------|-----------|
| List pop | At-most-once (gone after pop) |
| Stream + group + ack | **At-least-once** (can redeliver on crash) |

Handlers must be **idempotent** or use deduplication keys.

### 4.4 Streams are not a free deque

True deque semantics (pop from **either** end with symmetry) are natural on **Lists**. On **Streams**, append is easy (`XADD`); “pop from front” of the log is a **design problem** (IDs, trimming, consumer direction). Your README calls this out — expect extra indexing logic for `StreamDeque`, not just one command per `PopFront`.

### 4.5 Stream operations cheat sheet

| Goal | Command (conceptual) |
|------|----------------------|
| Publish | `XADD key * field value` |
| Create group | `XGROUP CREATE key group $ MKSTREAM` |
| Read new messages | `XREADGROUP GROUP g c BLOCK ms STREAMS key >` |
| Ack | `XACK key group id` |
| See lag | `XPENDING key group` |
| Limit growth | `XTRIM key MAXLEN ~ 10000` |

---

## 5. Choosing List vs Stream

| Question | List | Stream |
|----------|------|--------|
| Need replay after consume? | No | Yes |
| Need ack / retry / DLQ? | Build yourself | Built-in group + PEL |
| Lowest latency? | Yes | No |
| Deque both ends simply? | Yes | Harder |

---

## 6. Keys, memory, and ops hygiene

### Key naming

```
deque:{tenant}:{queueName}
```

- One key = one logical deque (one list or one stream).
- Multi-tenant: prefix by tenant; never share keys across tenants.

### Memory

- Redis is in-memory. Unbounded `RPUSH` fills RAM.
- **`maxmemory-policy`:** `allkeys-lru` can **evict queue items** — usually bad for task queues. Prefer **`noeviction`** (writes fail when full) or app-side max depth.
- Streams: use **`MAXLEN ~`** on `XADD` or periodic `XTRIM`.

### Commands to avoid in library code

| Command | Why |
|---------|-----|
| `KEYS *` | Blocks server; O(N) |
| `FLUSHDB` | Destroys data |

Use explicit key names from config.

---

## 7. Failures you should understand

| Scenario | List behavior | Stream behavior |
|----------|---------------|-----------------|
| Worker crashes after pop, before work | Message **lost** | Message stays in **PEL**, redelivered |
| Client timeout, server succeeded | Unknown if pop happened — **dangerous to retry pop** | May see duplicate on redelivery |
| Primary fails | Failover to replica; small replication lag window | Same + in-flight PEL entries |
| Redis down | Return connection error — **not** `ErrEmpty` | Same |

**Half-open failure:** always separate **infrastructure errors** from **`ErrEmpty`**.

---

## 8. go-redis mapping (when you code)

```go
// ListDeque PopFront (conceptual — you write the real version)
val, err := client.LPop(ctx, d.key).Bytes()
if err == redis.Nil {
    return nil, distributeddeque.ErrEmpty
}
if err != nil {
    return nil, fmt.Errorf("lpop: %w", err)
}
return val, nil
```

- Always pass **`ctx`**.
- Compare **`redis.Nil`** for empty list, not string matching.
- Wrap other errors with context.

---

## 9. Local practice

Run Redis locally (Docker example when you add `docker/`):

```bash
docker run -d --name redis -p 6379:6379 redis:7
redis-cli -h 127.0.0.1 PING
```

Experiment with the `redis-cli` snippets in §3 before wiring Go.

---

## 10. Suggested learning path

1. **`redis-cli`** — Lists: `LPUSH`/`RPUSH`/`LPOP`/`RPOP`/`LLEN`.
2. **`errors.go`** — wire `ErrEmpty` from `redis.Nil`.
3. **`config.go`** — key name, client options, timeouts.
4. **`list/deque.go`** — implement `ListDeque` only.
5. **Integration test** — one push/pop against real Redis.
6. **Streams** — read Redis docs on consumer groups; sketch `StreamDeque` on paper before coding.

Update [`AGENTS.md`](../AGENTS.md) when you complete each milestone.
