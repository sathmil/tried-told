# Design Decision 23: Tombstone-Aware Querying

Status: decided, Lexical index upgrade (in progress)
Date: 2026-07-29

## Decision

`bm25.FilterDeleted(results []Result, seg *diskindex.Segment, deleted Deleter) []Result`
- a **post-processing step**, not a parameter to `Search` itself.

## Why this can't live inside `Search`

Filtering requires resolving a result's local `DocID` back to a stable
`Passage.ID()` to check against the deletion log - and that mapping only
exists for segment-backed search. The in-memory Milestone 1 backend has no
concept of a stable passage identity at all (its DocIDs are ephemeral,
assigned per-build). Baking deletion-awareness into `Search` would force
every in-memory caller to pass something structurally meaningless to them.
Keeping scoring and deletion-filtering as separate composable steps keeps
`Search` genuinely backend-agnostic, which is the property `bm25.Index`
(design doc 21) exists to protect.

## `Deleter`: a minimal interface, not a `crawlstate` dependency

```go
type Deleter interface {
    IsDeleted(passageID string) bool
}
```

`*crawlstate.DeletionLog` already satisfies this exactly, with zero
adapter code needed - same pattern as `fetch.Deduper` matching
`*dedup.Registry`'s existing method shape for free. `bm25` depends only on
this small interface, not on `crawlstate` or its WAL-backed persistence
mechanism.

## Order preserved, not re-sorted

`FilterDeleted` removes entries but never reorders the remainder - the
input is already ranked by `Search`; filtering shouldn't second-guess that
ranking.

## Test worth calling out

`TestFilterDeleted_RealExtractedPassages` runs the real extractor, builds
a real segment, opens a real WAL-backed `DeletionLog`, searches, deletes
one of the actual returned results by its real `Passage.ID()`, and
confirms it - and only it - disappears from the filtered results. The
full real pipeline (extraction → segment → search → deletion → filtering)
proven together, not each piece in isolation.
