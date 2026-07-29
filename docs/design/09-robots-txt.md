# Design Decision 09: robots.txt Compliance

Status: decided, Crawler component (in progress)
Date: 2026-07-29

## Decision

Use `github.com/jimsmart/grobotstxt` (`internal/robots.Checker`) rather than
hand-rolling a parser - the first external dependency in this project.

**Why a library, not hand-rolled:** unlike the inverted index/BM25 (explicit
learning targets, built from scratch), robots.txt parsing is protocol
compliance with many small, easy-to-get-wrong edge cases (wildcard matching,
Allow/Disallow precedence, malformed files). Getting it subtly wrong isn't
just a bug - it's violating a site owner's actual stated wishes. Same
reasoning the project already applies to the ANN library choice: use a
mature implementation, understand it, be able to defend it.

**Why this specific library:** it's a faithful, function-for-function Go
port of Google's own official C++ robots.txt parser (Apache 2.0), the same
implementation RFC 9309 (the actual IETF standard for the Robots Exclusion
Protocol) is based on - passes the original test suite. Checked license and
maintenance status directly rather than assuming, per the lesson from
design doc 07.

## Fetch outcome policy

| Outcome | Result | Cached? |
|---|---|---|
| 200, parses fine | Real rules apply | Yes |
| 404 | No restrictions - crawl freely | Yes |
| Any other status, or fetch fails entirely | **Fail closed** - not allowed | **No** |

**Why fail closed on fetch failure, not fail open:** the project's founding
principle is "don't assume permission - confirm it" (the same reasoning
behind refusing to scrape ToS-prohibited sites). If robots.txt can't be
fetched, there is zero information about what the site allows; crawling
anyway would mean proceeding without confirmed permission.

**Why failures are never cached:** fail-closed doesn't have to mean
permanently blackholing a host over one bad request. The crawler already
needs retry-with-backoff for ordinary fetch failures (frontier task). By
simply not caching a `disallowAll` outcome, the next call to `Allowed`
naturally retries the fetch fresh - "not allowed *yet*," not "not allowed
forever" - without needing separate TTL/expiration logic.

## Test worth calling out

`TestChecker_FailureIsNotCachedSoLaterCallsRetry` uses a mock server that
fails the first request and succeeds on the second, and confirms the
`Checker` retries rather than staying stuck on the first failure - a direct
test of the fail-closed-but-not-forever policy, not just a smoke test.
