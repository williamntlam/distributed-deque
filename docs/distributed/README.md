# Distributed coordination — documentation index

This folder catalogs **ways to prevent race conditions and build distributed queues** without assuming “one big distributed mutex” is the only answer. It complements the hands-on v1 stack (`MemoryDeque` + `cmd/queued`).

**No implementation code lives here** — only design notes, comparisons, and optional future tracks.

---

## Start here

| Doc | Contents |
|-----|----------|
| [`race-condition-strategies.md`](race-condition-strategies.md) | Full catalog: mutex, OCC, event loops, CAS, append-only logs, CRDTs, partitioning |
| [`this-project.md`](this-project.md) | What **distributed-deque** uses today vs what each strategy would change |
| [`design-chooser.md`](design-chooser.md) | Checklist: strict FIFO vs pub/sub scale, when to pick which strategy |
| [`tracks/README.md`](tracks/README.md) | Per-strategy learning tracks (outline only) |

---

## Relationship to code in this repo

| Layer | Location | Strategy (v1) |
|-------|----------|----------------|
| In-process safety | `memory/deque.go` | `sync.Mutex` on one deque |
| Multi-process | `cmd/queued/main.go` | Single owner + HTTP; mutex on server |
| Clients | `curl` / any HTTP tool | No bundled Go client |

Future experiments described under `tracks/` are **not** scheduled in v1 unless you add them explicitly.

---

## Related docs

- [`../deque-guide.md`](../deque-guide.md) — deque semantics, Mode A/B ordering, broker pattern
- [`../../README.md`](../../README.md) — repository overview and layout
