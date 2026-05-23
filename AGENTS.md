# Agent context — distributed-deque

This file is the **canonical project memory** for humans and AI assistants. Cursor loads `.cursor/rules/project-memory.mdc` (`alwaysApply: true`) which points here. **Keep both in sync** when facts change.

Last updated: 2026-05-19

---

## Project summary

**distributed-deque** is a Go library exposing a **double-ended queue**. **v1** uses an in-memory **doubly-linked list** (`[]byte` per node, `head`/`tail`, `sync.Mutex`) in `MemoryDeque` for **O(1)** push/pop at both ends. **Distributed** behavior is learned by giving **one process** ownership of that deque and accessing it via **`RemoteDeque`** (HTTP/RPC client) — not Redis in v1.

| Implementation | Storage | When to use |
|----------------|---------|-------------|
| **MemoryDeque** | Doubly-linked list + mutex in one process | Tests, single binary, worker pools |
| **RemoteDeque** (planned) | Client → queue server owning `MemoryDeque` | Multiple processes, distribution challenge |

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

Early implementation. Treat **README** (including **Repository layout**) as the file contract until code lands.

| File / area | Status |
|-------------|--------|
| `errors.go` | **Done** — `ErrEmpty`, `ErrClosed`; `isEmpty` / `isClosed` |
| `deque.go` | **Scaffold** — `Deque` interface only |
| `config.go` | **Scaffold** — empty `Config`; fill in for `RemoteDeque` |
| `memory/node.go` | **Scaffold** — `node` type |
| `memory/deque.go` | **Scaffold** — `MemoryDeque` fields + `NewMemoryDeque`; **you** implement ops |
| `memory/deque_test.go` | **Scaffold** — empty test package |
| `memory/ring.go` | **Placeholder** — ring-buffer optimization |
| `remote/deque.go`, `remote/deque_test.go` | **Scaffold** — planned HTTP client |
| `cmd/queued/main.go` | **Scaffold** — empty `main` |
| `test/integration/remote_deque_test.go` | **Scaffold** — `integration` build tag |
| `list/`, `stream/`, `internal/redis/` | **Do not create** |

Full tree: see **Repository layout** in [`README.md`](README.md).

### Typed errors (current + planned)

| Error | Meaning |
|-------|---------|
| `ErrEmpty` | Pop on empty deque (non-blocking); not a client failure |
| `ErrClosed` | Client closed; no further ops on this instance |
| `ErrReadOnly`, `ErrTimeout` | Planned (see README roadmap) |

**API stance:** v1 `Deque` is **sync only**. **Later:** optional blocking pop + async client helpers (channels / goroutine wrappers); see README “Sync API now, async later”.

Callers should use `errors.Is(err, distributeddeque.ErrEmpty)` (export `IsEmpty` / `IsClosed` later if needed).

---

## Developer environment

- **OS:** Linux (WSL2), Ubuntu 24.04
- **Go installs seen in this project:**
  - `sudo apt install golang-go` → **1.22** at `/usr/bin/go`
  - Optional: `sudo apt install golang-1.24-go` → `/usr/lib/go-1.24/bin/go`
  - Optional: official tarball → `/usr/local/go/bin/go` (e.g. 1.26)
- **Common failure:** `go mod tidy` → `toolchain not available` when `go.mod` requires a newer Go than on `PATH`. Fix: lower `go` in `go.mod` to match `go version`, or install the required version and fix `PATH`.
- **Wrong artifact:** `go1.x.x.src.tar.gz` is source only; use `go1.x.x.linux-amd64.tar.gz` for a binary install.
- Do not store Go tarballs inside this repo.

---

## Learning mode (how to help the author)

The author is **building this library to learn**. Agents should default to **guide, not generate**.

| Prefer | Avoid (unless asked) |
|--------|----------------------|
| Step-by-step plan for the next file or function | Landing a full package in one shot |
| Example skeleton: imports, types, method stubs | Complete `MemoryDeque` / HTTP server in one shot |
| "Your `memory/deque.go` might look like…" with 15–30 lines max | Editing every stub file in the repo |
| Explaining *why* (mutex scope, error choice, distribution) | Opaque copy-paste solutions |
| Reviewing their code, compiler errors, test failures | Pre-writing integration tests for them |

**Escalation:** If they say *implement*, *write the file*, *apply*, or *fix it*, switch to hands-on coding.

**Good prompt habits for the author:** "Guide me through `deque.go`", "Show a skeleton only", "Review my diff", "How do I block on empty pop with context?"

---

## Cursor rules (topic-focused)

| Rule file | When it applies |
|-----------|-----------------|
| `.cursor/rules/project-memory.mdc` | Always |
| `.cursor/rules/go-conventions.mdc` | When editing `**/*.go` |
| `.cursor/rules/memory-patterns.mdc` | When editing `memory/`, `remote/`, `cmd/`, tests |

## Conventions for agents

1. Read `README.md` before designing APIs.
2. Respect **learning mode** above; keep agent-written diffs small.
3. Do not run `git commit` unless asked.
4. After meaningful changes (new packages, errors, Go version), update **this file** and `.cursor/rules/project-memory.mdc`.

---

## Chat history notes

- Renamed from `distributed-queue` → **distributed-deque** (bidirectional ends, not strict FIFO queue).
- Docs pivoted from **Redis-first** to **in-memory list first**, with optional HTTP queue server for distribution (2026-05-19).
- `errors.go` distinguishes empty deque vs closed client — not the same as “temporarily unavailable.”
