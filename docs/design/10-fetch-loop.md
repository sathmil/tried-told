# Design Decision 10: Fetch Loop

Status: decided, Crawler component (in progress)
Date: 2026-07-29

## Decision

`internal/fetch` has two pieces:

- **`Fetcher.Fetch(url)`** - retrieves one URL: normalizes it, checks dedup,
  checks robots.txt, retries transient failures with exponential backoff,
  and follows redirects *manually* rather than trusting `net/http`'s
  default auto-follow.
- **`Crawl(ctx, frontier, fetcher, workers, out)`** - a worker pool: `n`
  goroutines pull from the `Frontier`, call `Fetcher.Fetch`, and send
  results on `out`, until the frontier drains or `ctx` is cancelled.

## Three decisions, and why each matters

**1. Workers sleep exactly until the next host is ready** (`Frontier.NextReadyAt`),
not a fixed poll interval and not a busy-loop. Busy-looping burns CPU for no
reason; a fixed interval is arbitrary and either wastes time or reacts late.
Since the frontier's heap already knows the exact soonest ready time, workers
can sleep precisely that long and wake up exactly when there's real work.

**2. Retries only 5xx, 429, and network-level failures** - not a plain 404,
which is a definitive "this doesn't exist," not a transient condition worth
retrying. Exponential backoff (`BaseDelay` doubling up to `MaxDelay`) over
`MaxAttempts` total tries.

**3. Redirects are intercepted manually**
(`CheckRedirect: return http.ErrUseLastResponse`), not auto-followed. A
redirect can point to a **different host entirely**, which may have
completely different robots.txt rules. Auto-following would silently fetch
that host's content without ever checking *its* permissions or running
dedup on the actual final URL - a real compliance gap, not just a style
preference. Every hop (original URL and each redirect target) independently
re-runs normalize -> dedup -> robots.txt -> fetch.
`TestFetcher_RedirectTargetDisallowedByRobotsIsCaught` proves this directly:
a redirect to a disallowed path is caught, not silently served.

## Scope boundaries (deliberate, not oversights)

- No link extraction / dynamic frontier growth yet - `Crawl` runs against a
  fixed, pre-seeded set of URLs and stops when the frontier drains. Parsing
  fetched HTML for new links belongs to the extraction milestone.
- Retry backoff testing uses tiny **real** sleeps (1-10ms), not an injected
  fake clock like the Frontier's cooldown tests. The property under test
  here (retries the right number of times, eventually succeeds or gives up)
  doesn't need precise timing verification the way the Frontier's exact
  cooldown boundary did - real sleeps are simpler and sufficient.
- The worker pool's wait computation uses real `time.Now`/`time.Until`,
  consistent with `Crawl` always being run against a real-clock `Frontier`
  in practice (and in its own integration test) - not a general-purpose
  injectable clock, which would be over-engineering for this scope.

## Test worth calling out

`TestCrawl_RespectsPolitenessAcrossConcurrentWorkers` runs 4 concurrent
workers against two real local mock servers and verifies, from actual
recorded request timestamps, that each host's requests stay at least
`delay` apart - proving politeness holds under genuine concurrent
execution, not just in the Frontier's own isolated unit tests. The whole
suite also passes under `go test -race`, the first real concurrency in
this project.
