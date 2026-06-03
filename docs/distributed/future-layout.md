# Hypothetical future layout (documentation only)

**Nothing below exists in the repo yet.** This file records where new code *might* live if you extend beyond v1 (`memory` + `cmd/queued`). Do not create these packages until you commit to a track.

---

## Current (v1) — implemented

```
distributed-deque/
├── errors.go
├── deque.go
├── memory/
│   ├── node.go
│   ├── deque.go
│   ├── deque_test.go
│   └── ring.go              # later optimization
└── cmd/
    └── queued/
        └── main.go          # HTTP server, owns MemoryDeque
```

---

## Track A — Harden central broker (still mutex)

```
cmd/queued/
├── main.go
├── handlers.go              # optional split
└── shutdown.go              # graceful drain

docs/distributed/tracks/01-mutex-central-owner.md
```

No new packages; same strategy, better ops.

---

## Track B — Partitioned HTTP brokers

```
cmd/
├── queued/                  # router or single shard
└── queued-shard/            # optional: one deque per shard instance

internal/
└── routing/                 # hash tenant_id → shard URL (doc-only sketch)
    └── README.md

docs/distributed/tracks/07-partitioning.md
```

Multiple processes — **avoid split-brain** with explicit routing; not “two queued on same deque.”

---

## Track C — Lease / OCC metadata (alongside deque)

```
internal/
└── lease/                   # versioned claim table (future)
    ├── README.md
    ├── store.go             # (future) interface
    └── memory_store.go      # (future) in-RAM for tests

cmd/queued/
└── handlers.go              # pop → lease; ack endpoint (future)
```

Deque may stay mutex-backed; **leases** use OCC/CAS semantics on separate records.

---

## Track D — Append-only log (separate subsystem)

Not a drop-in replacement for `MemoryDeque`.

```
log/                         # (future) new module — name TBD
├── README.md
├── segment.go
└── consumer_offset.go

cmd/
└── logd/                    # (future) log server binary
    └── main.go

docs/distributed/tracks/05-append-only-log.md
```

Different `Deque` implementation or new interface — design TBD before any files.

---

## Track E — External store backend

```
internal/
└── store/                   # (future) etcd/SQL adapter
    └── README.md

docs/distributed/tracks/04-atomic-cas.md
```

Would implement **claim** via store CAS, not linked-list mutex.

---

## Docs-only tree (created now)

```
docs/distributed/
├── README.md
├── race-condition-strategies.md
├── this-project.md
├── design-chooser.md
├── future-layout.md          # this file
└── tracks/
    ├── README.md
    ├── 01-mutex-central-owner.md
    ├── 02-optimistic-concurrency.md
    ├── 03-single-threaded-shard.md
    ├── 04-atomic-cas.md
    ├── 05-append-only-log.md
    ├── 06-crds.md
    └── 07-partitioning.md
```

---

## Rules if you add code later

1. Do not create `list/`, `stream/`, `internal/redis/` (legacy names).
2. Prefer new tracks in **new directories** with a `README.md` before Go files.
3. Update root `README.md` layout and `AGENTS.md` when a track lands.
