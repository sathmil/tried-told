# Design Decision 11: Persistent, Resumable Frontier + Raw Content Storage

Status: decided, Crawler component (in progress)
Date: 2026-07-29

## Decision

**Pure write-ahead log (WAL)**, not full-state snapshots and not a
snapshot+WAL hybrid - chosen specifically because the project isn't at real
crawl volume yet (still testing against local mocks), and both alternatives
solve problems (log growth, recovery time) that only bite at a scale not
yet reached. Consistent with the "design for the next stage, not the one
after" reasoning used throughout this project.

`internal/wal` is a generic, reusable primitive: `Log[T]` appends
JSON-encoded entries (one per line, fsynced before returning), and
`Replay[T]` reads every entry back on startup, calling a function per
entry to reconstruct state. A crash mid-write can only ever corrupt the
*last* line (everything before it was already fsynced) - `Replay` stops at
the first line it can't parse rather than erroring the whole replay, since
that's expected recovery behavior, not corruption.

Three separate logs share this one primitive, each with exactly one event
type:

| Log | Event | Purpose |
|---|---|---|
| Frontier (`internal/crawlstate`) | `EnqueueEntry` (host, url) | Replay reconstructs pending work |
| Dedup (`internal/crawlstate`) | `SeenEntry` (url) | Replay reconstructs "already done" |
| Content (`internal/fetch`, optional) | `ContentRecord` (url, timestamp, hash, body) | Raw archive of fetched pages |

## The key insight: the frontier log doesn't need a "completed" event

The frontier log only ever records "enqueued" - never "dequeued" or
"completed." On restart, replay re-adds *every* URL ever discovered,
including ones already fetched before the crash. That's fine, because
`Fetcher.Fetch`'s very first step is `dedup.SeenOrAdd` - if the dedup log
has *also* been replayed, an already-fetched URL is correctly recognized as
seen and skipped (`ErrAlreadySeen`), without ever hitting the network again.

This means two independent, single-purpose logs are simpler and sufficient,
rather than one unified log with multiple event types and replay-ordering
logic: the frontier doesn't need to know or care what's "done" - that's
entirely dedup's job, and dedup already has to persist regardless.

`TestResumability_AlreadyFetchedURLIsSkippedAfterRestart` proves this
directly: fetches one of two enqueued URLs, "crashes" (closes both logs
without fetching the second), reopens both logs fresh, and confirms the
already-fetched URL is skipped via `ErrAlreadySeen` while the genuinely new
one still fetches normally - checked against real request counts on a mock
server, not just return values.

## Content storage: JSONL, not WARC

Chosen because nothing in this project yet interoperates with external WARC
tooling, and everything else already leans on plain JSONL. Revisit if/when
real WARC interop (e.g. feeding files into existing web-archive tooling)
becomes a real requirement - not before.

## Interface wrinkle worth noting

`Fetcher` depended on the concrete `*dedup.Registry` type, so
`PersistentRegistry` couldn't substitute in without a change. Fixed by
having `Fetcher` depend on a minimal `Deduper` interface
(`SeenOrAdd(string) bool`) instead - `*dedup.Registry` already satisfies it
with zero changes, and existing fetcher tests kept passing unmodified.
`PersistentRegistry.SeenOrAdd` panics on a WAL append failure rather than
returning an error: a failing disk write is already a halt-worthy
operational problem here, not something to gracefully thread through every
caller.

## Deliberately out of scope

- **Per-host cooldown timing is not persisted.** On restart, every host's
  cooldown simply resets (since replay just calls `Add`, and a fresh
  `Frontier` has no memory of pre-crash fetch times). Accepted as a minor,
  bounded politeness cost - worst case, one host gets fetched slightly
  sooner after a restart than its full cooldown would have allowed, not a
  correctness issue.
- **No snapshotting/log compaction yet.** Revisit once log replay time or
  disk usage actually becomes a measured problem, per the same
  design-for-current-scale reasoning as the WAL-vs-hybrid choice above.
