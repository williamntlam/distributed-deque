# Agent context — distributed-deque

This file is the **canonical project memory** for humans and AI assistants. Cursor loads `.cursor/rules/project-memory.mdc` (`alwaysApply: true`) which points here. **Keep both in sync** when facts change.

Last updated: 2026-05-29

---

## Project summary

**distributed-deque** is a Go library exposing a **double-ended queue**. **v1** uses an in-memory **doubly-linked list** (`[]byte` per node, `head`/`tail`, `sync.Mutex`) in `MemoryDeque` for **O(1)** push/pop at both ends. **Distributed** behavior is learned via **`cmd/queued`**: one process owns the deque and exposes HTTP (`POST /push`, `GET /pop`). Other processes use **HTTP clients** (e.g. `curl`) — there is **no** `remote` Go package. Redis is **not** in scope.

| Piece | Role |
|-------|------|
| **MemoryDeque** | Doubly-linked list + mutex in one process |
| **cmd/queued** | Queue server — only owner of canonical deque; HTTP API |

Full design: [`README.md`](README.md). Learning doc: [`docs/deque-guide.md`](docs/deque-guide.md).

---

## Repository facts

| Item | Value |
|------|--------|
| Go module | `github.com/williamntlam/distributed-deque` |
| Package | `distributeddeque` |
| `go.mod` Go version | `1.26` (must match installed `go`; see below) |
| Primary dependency | Standard library for v1 (`sync`, `context`, …) |

---

## Code status

| File / area | Status |
|-------------|--------|
| `errors.go` | **Done** — `ErrEmpty`, `ErrClosed` |
| `deque.go` | **Done** — `Deque` interface |
| `memory/node.go`, `memory/deque.go` | **Done** — linked deque + ops |
| `memory/deque_test.go` | **Done** — FIFO + concurrent tests |
| `memory/ring.go` | **Placeholder** — ring-buffer optimization |
| `cmd/queued/main.go` | **Done** — HTTP server, owns `MemoryDeque` |
| `list/`, `stream/`, `internal/redis/`, `remote/` | **Do not create** |

Full tree: README **Repository layout**.

### Typed errors

| Error | Meaning |
|-------|---------|
| `ErrEmpty` | Pop on empty deque (non-blocking) |
| `ErrClosed` | Deque closed after `Close()` |
| `ErrReadOnly`, `ErrTimeout` | Planned (see README roadmap) |

**HTTP mapping (server):** `ErrEmpty` → 204 on `GET /pop`; `ErrClosed` → 503.

---

## Developer environment

- **OS:** Linux (WSL2), Ubuntu 24.04
- **Go:** align `go.mod` with `go version` on `PATH` (see README / project-memory for install notes).
- Do not store Go tarballs inside this repo.

---

## Learning mode (how to help the author)

The author is **building this library to learn**. Agents should default to **guide, not generate**.

| Prefer | Avoid (unless asked) |
|--------|----------------------|
| Step-by-step plans, skeletons | Full packages in one shot |
| Explain *why* (mutex, HTTP status codes) | Opaque copy-paste |
| Review their code and errors | Pre-writing everything for them |

**Escalation:** *implement*, *apply*, *fix it* → hands-on coding.

---

## Cursor rules

| Rule file | When it applies |
|-----------|-----------------|
| `.cursor/rules/project-memory.mdc` | Always |
| `.cursor/rules/go-conventions.mdc` | `**/*.go` |
| `.cursor/rules/memory-patterns.mdc` | `memory/`, `cmd/` |

## Conventions for agents

1. Read `README.md` before designing APIs.
2. Respect **learning mode**; keep diffs small.
3. Do not `git commit` unless asked.
4. Update this file and `project-memory.mdc` when layout or status changes.

---

## Chat history notes

- Renamed `distributed-queue` → **distributed-deque**.
- Docs: in-memory list first, then HTTP queue server (not Redis-first).
- 2026-05-29: Removed `remote/` and `config.go`; distribution via `cmd/queued` + curl/HTTP only.
