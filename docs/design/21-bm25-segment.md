# Design Decision 21: BM25 Against Either Backend

Status: decided, Lexical index upgrade (in progress)
Date: 2026-07-29

## Decision

`bm25.Search` now takes a small `bm25.Index` interface (`Postings`, `N`,
`AvgDocLen`, `DocLen`) instead of the concrete `index.Index` struct. Two
adapters satisfy it: `WrapInMemory(index.Index)` (Milestone 1's backend)
and `WrapSegment(*diskindex.Segment)` (the new disk-backed backend). The
same scoring code now runs against either.

## Why adapters, not methods directly on `index.Index`

`index.Index` is a struct with **public fields** (`N`, `AvgDocLen`,
`DocLen`, `Postings`). Go doesn't allow a method and a field with the same
name on one type, so `index.Index` can't directly grow `N()`/`AvgDocLen()`
methods without renaming its existing fields - which would break every
existing caller (tests, `cmd/eval`, `cmd/server`) for no real benefit.
Small wrapper types (`inMemoryIndex`, `segmentIndex`) sidestep this
entirely: `index.Index` and `diskindex.Segment` are both untouched, and
the adaptation lives in one place (`internal/bm25/adapters.go`) that's
free to depend on both lower-level packages, since `bm25` already sits
above them in the dependency graph.

## `Posting` is deliberately backend-agnostic

`bm25.Posting{DocID, Freq}` doesn't try to match either backend's native
posting shape - the in-memory index stores `Freq` directly; a disk segment
stores positions and `Freq` is derived as `len(Positions)`. Each adapter
does that translation once, at the boundary, so `Search` itself never
needs to know or care which backend produced a posting.

## Real, not superficial, proof of equivalence

The critical test isn't "the segment-backed search returns something" -
it's `TestSearch_InMemoryAndSegmentAgreeOnRealExtractedData`: the *same*
real extracted passages, indexed both ways, must produce **identical
scores and rankings** through both backends. This is a stronger claim than
"both work" - it proves the disk-backed path faithfully reproduces the
same, already-validated scoring behavior, not just that it doesn't crash.

## Scope boundary

Passage-level tombstones (`crawlstate.DeletionLog`) still aren't checked
during search - a deleted passage would still be returned if a segment
still contains it. That's the next natural piece (tombstone-aware
querying), not bundled into this one.
