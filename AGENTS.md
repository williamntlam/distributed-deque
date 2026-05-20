# Agent context — distributed-deque

This file is the **canonical project memory** for humans and AI assistants. Cursor loads `.cursor/rules/project-memory.mdc` (`alwaysApply: true`) which points here. **Keep both in sync** when facts change.

Last updated: 2026-05-19

---

## Project summary

**distributed-deque** is a Go library exposing a **double-ended queue** whose state lives in **Redis**, so many processes share one logical deque per key.

| Engine | Redis primitive | When to use |
|--------|-----------------|-------------|
| **ListDeque** | Lists (`LPUSH`/`RPUSH`/`LPOP`/`RPOP`, blocking variants) | Fast, ephemeral task routing |
| **StreamDeque** | Streams (`XADD`, consumer groups, `XACK`) | Durability, replay, at-least-once |

Full design: [`README.md`](README.md).

---

## Repository facts

| Item | Value |
|------|--------|
| Go module | `github.com/williamntlam/distributed-deque` |
| Package | `distributeddeque` |
| `go.mod` Go version | `1.26` (must match installed `go`; see below) |
| Primary dependency | `github.com/redis/go-redis/v9` |

---

## Code status

Early implementation. Treat **README** as the API/design contract until types land in code.

| File / area | Status |
|-------------|--------|
| `errors.go` | **Implemented** — `ErrEmpty`, `ErrClosed`; unexported `isEmpty` / `isClosed` |
| `deque.go`, `config.go`, `list/`, `stream/`, `internal/`, examples, tests | **Not implemented** (stubs or absent) |

### Typed errors (current + planned)

| Error | Meaning |
|-------|---------|
| `ErrEmpty` | Pop on empty deque (non-blocking); not a client failure |
| `ErrClosed` | Client closed; no further ops on this instance |
| `ErrReadOnly`, `ErrTimeout` | Planned (see README roadmap) |

Callers should use `errors.Is(err, distributeddeque.ErrEmpty)` (export `IsEmpty` / `IsClosed` later if needed).

---

## Developer environment

- **OS:** Linux (WSL2), Ubuntu 24.04
- **Go installs seen in this project:**
  - `sudo apt install golang-go` → **1.22** at `/usr/bin/go`
  - Optional: `sudo apt install golang-1.24-go` → `/usr/lib/go-1.24/bin/go`
  - Optional: official tarball → `/usr/local/go/bin/go` (e.g. 1.26)
- **Common failure:** `go mod tidy` → `downloading go1.24` / `toolchain not available` when `go.mod` requires a newer Go than on `PATH`. Fix: lower `go` in `go.mod` to match `go version`, or install the required version and fix `PATH`.
- **Wrong artifact:** `go1.x.x.src.tar.gz` is source only; use `go1.x.x.linux-amd64.tar.gz` for a binary install.
- Do not store Go tarballs inside this repo.

---

## Conventions for agents

1. Read `README.md` before designing APIs or Redis command usage.
2. Keep changes minimal; one concern per PR/commit when the user commits.
3. Do not run `git commit` unless asked.
4. After meaningful changes (new packages, errors, Go version), update **this file** and `.cursor/rules/project-memory.mdc`.

---

## Chat history notes (from setup)

- Renamed from `distributed-queue` → **distributed-deque** (bidirectional ends, not strict FIFO queue).
- Chose **Go 1.22** via apt when 1.24 auto-download failed; later `go.mod` may target **1.26** if `/usr/local/go` is on `PATH`.
- `errors.go` distinguishes empty deque vs closed client — not the same as “temporarily unavailable.”
