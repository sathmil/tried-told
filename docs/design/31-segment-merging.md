# Design Decision 31: Segment Merging

Status: decided, Lexical index upgrade (complete)
Date: 2026-07-30

## Decision

`diskindex.MergeSegments(segs []*Segment, deleted Deleter, path string) error`
combines any number of existing segments into one new segment file,
permanently dropping any passage `deleted` reports as tombstoned - its
postings, doc-length entry, and ID-mapping entry are simply left out of
the output, not merely hidden from it.

## Why merging exists: reclaiming space, not just query performance

`crawlstate.DeletionLog` (design doc 18) only filters deleted passages
out of query results - the postings for a deleted passage stay on disk,
still contributing to file size and dictionary term counts, forever,
until something rewrites the segment without them. Merging is that
something: because merging already has to decode and re-encode every
segment's postings (see below), it's the one natural point to also drop
tombstoned passages entirely. That's the actual reason production search
engines merge segments periodically - not purely a query-latency
optimization from having fewer files to touch, though that's a real
secondary benefit too.

Two other approaches were considered and rejected: merging that ignores
deletion status entirely (simpler, no `Deleter` dependency, but the
index never actually shrinks - tombstones accumulate forever and query-time
filtering cost never decreases, defeating the point of merging
periodically); and post-hoc rewriting a segment in place to strip
deletions without combining it with anything else (solves reclamation
but not the separate small-segment-proliferation problem, so you'd need
two mechanisms instead of one).

## Why merging can't just reuse BuildSegment

`extract.Passage.Text` is never stored in a segment - only postings, doc
lengths, and the ID mapping survive extraction (design doc 20).
`BuildSegment` gets its postings by tokenizing `Passage.Text` fresh; a
merge has no text to tokenize, only already-encoded postings from
existing segments. So `BuildSegment`'s file-writing tail (header layout,
checksum, section writes) was pulled out into a shared `writeSegment`
helper that operates on the same intermediate representation
(`terms`, `termDocPositions`, `docLens`, `passageIDs`) either function
can produce - `BuildSegment` by tokenizing, `MergeSegments` by decoding
and recombining. This avoided duplicating the encode/write logic between
the two paths rather than writing a second, parallel file writer.

## The renumbering trick that avoids a real k-way merge

Each segment's surviving (non-tombstoned) passages get fresh global IDs
assigned in segment order - segment 0's survivors first, then segment
1's, and so on. Because that assignment is monotonic within each
segment (increasing old local ID always produces an increasing new
global ID), and segment 0's whole ID range sits entirely below segment
1's, concatenating each segment's remapped-and-filtered postings for a
term in segment order already produces a fully merged, correctly-ordered
result - not because concatenation happens to be sorted by luck, but
because `writeSegment`'s `encodePostings` re-sorts by DocID from the
map keys regardless. Renumbering by segment order removes any need for
an actual interleaved merge step; the existing sort is enough.

## Real proof, not just wiring

`TestMergeSegments_DropsTombstonedPassages` is the test that actually
justifies the design: a term appearing only in a deleted passage's text
must be entirely absent from the merged dictionary afterward (`DocFreq`
0, not present in `Terms()`), not merely unreachable via the deletion
log - proving space is genuinely reclaimed, not just re-hidden.
`TestMergeSegments_PreservesPositionsForPhraseSearch` proves positions
survive renumbering intact (phrase search still works on the merged
segment), not just doc-level frequencies. `TestMergeSegments_ThreeWayMergeSumsAllSegments`
confirms the design handles any number of segments in one pass, not
just pairs - a real choice, since a pairwise-only merge would have been
simpler to reason about but would need `log(n)` repeated merges to
combine n segments instead of one.

## Deliberately out of scope

- **When/how merging is triggered.** No policy exists yet for deciding
  when to merge (segment count threshold, size-tiered merging like an
  LSM tree, a scheduled job) - `MergeSegments` is a tested primitive a
  caller invokes explicitly, the same "prove the mechanism, defer the
  policy" split already used for the HNSW index's build/query vs. its
  update cadence.
- **Concurrency during merge.** Nothing here addresses swapping a
  `MultiSegment`'s constituent segments atomically while queries are in
  flight against the old ones - this only produces a new, correct
  segment file; wiring it into a live, continuously-updated index is
  separate work.
