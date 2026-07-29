# Design Decision 18: Deletion & Re-indexing Support

Status: decided, Extraction component (in progress)
Date: 2026-07-29

## Decision

**Content-based passage identity** (`Passage.ID()`, a SHA-256 of
`SourceURL + "\x00" + Text`) plus a **WAL-backed deletion tombstone log**
(`crawlstate.DeletionLog`) - same append-only, single-event-type pattern
already used for the frontier and dedup logs.

## Passage identity: content-based, not position-based

Two ways to identify a passage stably across re-extractions:
position-based (`SourceURL` + index within the page) or content-based
(hash of `SourceURL` + text). Chose content-based: a deletion request is
naturally about removing a specific *stated experience*, not "whatever
happens to occupy slot 3" - and if the underlying text changes (a user
edits their review), that's arguably a new state worth re-checking, not
something that should be silently treated as still being "the same"
passage. Re-extracting genuinely unchanged content is idempotent (same
ID every time); a real text edit naturally produces a different ID.

**Correctness detail:** the two fields are joined with a `"\x00"`
separator, not plain concatenation - without it,
`SourceURL="http://a.com/b", Text="c"` and
`SourceURL="http://a.com", Text="/bc"` would concatenate to the identical
string and collide. Covered directly by
`TestPassage_ID_NoConcatenationCollisionAcrossFieldBoundary`.

## Deletion: a tombstone log, not immediate physical removal

`DeletionLog` records "this passage ID was deleted" as an append-only
event, replayed into an in-memory set on startup - not different in kind
from the frontier's enqueue log or dedup's seen log. No "undelete" event
exists; deletions are permanent by design, matching the project's
"avoid unnecessary retention" stance. Actual physical removal from any
index/storage built later is a downstream concern (a future index-builder
checks `IsDeleted` and excludes matching IDs) - the standard IR pattern of
logical delete now via tombstone, physical delete later during whatever
rebuild/compaction naturally happens next, same as Lucene/LSM-tree systems.

## Deferred: full re-indexing / content-change detection

Detecting that re-crawled content has genuinely *changed* (not just
"different ID because content-based identity says so") and triggering
reprocessing needs an actual index-building pipeline consuming real
`Passage`s to hook into - doesn't exist yet (Milestone 1's `BuildIndex`
doesn't consume extracted passages at all). Half the infrastructure is
already in place (`ContentHash` on `fetch.ContentRecord`); the rest is a
natural next step once a real indexing pipeline exists, not before -
consistent with every other "defer until the consuming system exists"
decision this session (generic boilerplate fallback, shared free-text
enrichment step).

## Test worth calling out

`TestDeletionLog_ExcludesRealDeletedPassageFromRealExtractedSet` extracts
real passages from the real fixture, deletes one by its real computed ID,
and confirms exactly that one - and only that one - is excluded from the
real remaining set. Not a synthetic ID string in isolation; the actual
extraction-to-deletion path, proven together.
