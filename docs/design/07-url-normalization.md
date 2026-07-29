# Design Decision 07: URL Normalization

Status: decided, Crawler component (in progress)
Date: 2026-07-29

## Decision

`internal/normalize.URL` canonicalizes a URL string so equivalent spellings
collapse to the same dedup key:

- lowercase scheme and host (paths stay case-sensitive - servers generally
  treat them that way)
- strip the default port (`:80` on http, `:443` on https)
- strip the fragment (the server never receives it)
- decode percent-encoded **unreserved** characters (`ALPHA` / `DIGIT` /
  `- . _ ~`); uppercase the hex digits of any escape left in place
- resolve `.`/`..` path segments
- sort query parameters by key

**Deliberately left unmerged**, per the earlier "be safe" decision: trailing
slash presence, `www` vs. non-`www` hosts, and tracking query parameters.
Any true duplicates these miss are expected to be caught later by
near-duplicate content detection (MinHash/SimHash, extraction stage), not
guessed at the URL-string level.

## The bug this caught (twice)

**First attempt was wrong twice**, both caught by tests written specifically
to verify behavior rather than assume it:

1. Assumed Go's `net/url` would normalize percent-encoding automatically on
   reserialization. A test proved otherwise: `net/url` keeps the *original*
   escaped path (`RawPath`) for round-trip fidelity and prefers it verbatim
   over re-deriving a canonical form from the decoded `Path`.
2. The obvious fix - clear `RawPath` so `String()` recomputes from `Path` -
   was itself wrong, and more seriously so: `Path` is *fully* decoded by
   `net/url`, including reserved characters, so an encoded slash (`%2F`)
   and a real `/` separator are indistinguishable once in `Path`. Clearing
   `RawPath` silently turned `%2F` into a real path separator, changing the
   URL's actual segment structure - not just its spelling.

**Fix:** operate on the *escaped* path string directly (`u.EscapedPath()`,
captured before any change), decode only unreserved characters (which can
never be structurally meaningful, so decoding them is always safe regardless
of order), leave reserved-character escapes in place (uppercased), then run
dot-segment resolution on that same escaped string - since a still-escaped
`%2F` contains no literal `/` byte, it's correctly never mistaken for a
separator during cleanup. `RawPath` and `Path` are then set from this same
normalized string so `net/url`'s internal consistency check accepts it.

Covered by tests including one that would have caught the second bug
directly: `a%2Fb/c/../d` -> `a%2Fb/d` (the encoded slash stays one atomic
segment; `..` cancels only the adjacent real segment, never reaching into
it).

## Why this matters as a lesson

Both bugs were "sounds right in my head" reasoning about a standard
library's behavior that turned out to be wrong on the first real test. The
fix wasn't "be more careful" in the abstract - it was writing a test that
would fail if the assumption was wrong, before trusting the assumption.
