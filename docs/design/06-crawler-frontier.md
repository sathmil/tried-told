# Design Decision 06: Crawler Frontier

Status: decided, Crawler component (in progress)
Date: 2026-07-29

## Decision

Per-host FIFO queues, scheduled by a min-heap of hosts keyed on
next-allowed-fetch time (`internal/frontier`).

- `Add(host, url)` appends to that host's queue; if the host has no pending
  heap entry, one is created.
- `Next()` pops the host with the soonest `nextAllowed` time, dequeues one
  URL from its queue, and (if more URLs remain for that host) re-pushes it
  with `nextAllowed = now + delay`. Returns `ok=false` if the frontier is
  empty or every host is still in its politeness cooldown - the caller
  should wait and retry, not treat this as permanently done.

## Alternatives considered

- **Single global FIFO queue.** Rejected: nothing stops several URLs from
  the same host landing back-to-back, so a worker could hammer one host
  while every other host's work sits idle behind it.
- **Round-robin over per-host queues with a fixed sleep between rounds.**
  Rejected: wastes time. If host A allows 1 req/sec and host B allows
  1 req/min, a fixed round period forces every host to move at the slowest
  host's rhythm, even when faster hosts have been ready the whole time.
- **Min-heap of hosts by ready-time (chosen).** A worker always pops
  whichever host is soonest-ready; if several hosts are already overdue it
  takes the most-overdue one and never idles as long as *any* host is ready.
  It only blocks when the minimum ready-time in the whole heap is still in
  the future - i.e. literally every host is in cooldown.

## Testability: injected Clock

`Frontier` takes a `Clock` (`func() time.Time`) instead of calling
`time.Now()` directly, so tests can advance a fake clock instantly rather
than sleeping in wall-clock time to verify politeness-delay behavior.

## Bug caught before shipping: cooldown must survive an empty queue

First draft only remembered a host's cooldown while it had a live heap
entry. Once a host's queue fully drained, its heap entry was removed - so if
new work arrived for that host shortly after, `Add` treated it as
immediately eligible, silently violating the politeness delay. Fixed by
tracking `lastFetch[host]` independently of queue/heap membership, so
`Add` can always compute `max(now, lastFetch[host]+delay)` regardless of
whether the host currently has any queued work. Covered by
`TestFrontier_DrainedHostGettingNewWorkStillRespectsCooldown`.
