# Design Decision 24: Multi-Segment Querying (Incremental Indexing)

Status: decided, Lexical index upgrade (in progress)
Date: 2026-07-29

## Decision

`diskindex.MultiSegment` combines several already-open `*Segment`s into
one queryable unit with a unified global DocID space. Incremental
indexing itself needed no new code - segments are already immutable and
self-contained, so a new batch of passages just becomes a new segment file
via the existing `BuildSegment`. The actual gap was that nothing could
*query* more than one segment at once; `MultiSegment` closes that.

## Global ID translation

Each constituent segment keeps its own local IDs (0..N-1). `MultiSegment`
assigns each segment a `base` offset (the first global ID it owns) and
translates in both directions: `locate(globalID)` finds the owning segment
via binary search over the (already-ascending) base offsets, and
`TermPostings`/`PhraseSearch` add each segment's base back onto its local
IDs when combining results. Same idea as Lucene's composite reader doc-base
scheme.

## One `Queryable` interface, not a separate wrapper per type

`Segment` and `MultiSegment` ended up with the identical method set
(`N`, `AvgDocLen`, `DocLen`, `PassageID`, `TermPostings`, `PhraseSearch`),
so rather than write parallel `WrapSegment`/`WrapMultiSegment` adapters in
`bm25`, a single `diskindex.Queryable` interface lets the existing
`WrapSegment(seg diskindex.Queryable) Index` handle both for free - one
segment or ten, `Search` doesn't need to know or care.

`bm25.FilterDeleted` was generalized the same way: it now takes a minimal
`PassageIDResolver` interface (just `PassageID(docID int) string`) instead
of the concrete `*diskindex.Segment` it originally required, so tombstone
filtering works identically whether results came from one segment or a
`MultiSegment` spanning several.

## Test worth calling out

`TestSearch_FindsResultsAcrossIncrementallyAddedSegments` is the actual
point being proven, not an implementation detail: two segments built at
two separate times (no rebuild of the first when the second is written)
are searched together as one `Search` call and both contribute results.
`TestFilterDeleted_WorksAcrossMultiSegment` deletes a passage specifically
from the *second* segment (non-zero base offset) to prove ID translation
is correct at a boundary, not just for the trivial first-segment case.

## Deliberately out of scope

Segment **merging** (combining several segments' raw postings into one new
segment, and physically dropping deleted passages in the process) is a
separate, harder problem - it needs a postings-level merge algorithm, not
just query-time combination, and wasn't built in this pass. `MultiSegment`
makes an unbounded number of segments *queryable*; it doesn't keep that
number bounded over time. That's the next piece, not this one.
