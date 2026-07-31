# Design Decision 35: Combining the Synthetic and Real Corpora

Status: decided, Real-source sourcing (complete)
Date: 2026-07-30

## Decision

`cmd/server` now serves both corpora at once: the synthetic demo corpus
(metadata-rich, small) and the real crawled corpus (genuinely real text,
no structured metadata) are combined into one unified lexical index via
`diskindex.NewMultiSegment` and one unified semantic index via two
`semantic.Index.Add` calls into the same graph. Nothing was replaced -
every existing synthetic passage is still searchable, real passages are
simply added alongside them.

## Why combine rather than replace

Real crawled content has no structured `Product`/`SkinTone`/`Climate`
tags - real blog HTML doesn't carry that markup, and nothing invents
what isn't there (design doc 33). Replacing the synthetic corpus
entirely would have thrown away the one thing it has that real content
doesn't: a small set of passages demonstrating metadata-filtered search.
Combining keeps both - and is also a more honest model of what a real
corpus actually looks like over time: some content richly tagged, some
not, growing incrementally rather than replaced wholesale.

## Why this needed no new mechanism, only new wiring

Both pieces of infrastructure this required already existed, built for
exactly this purpose:

- `diskindex.MultiSegment` (design doc 23) presents several segments as
  one unified `DocID` space - it exists specifically because "a new
  segment for newly-arrived passages is pointless if nothing queries it
  alongside the existing ones." Combining the synthetic and real
  segments is precisely that scenario, just with the two segments coming
  from different corpora instead of different crawl batches.
- `semantic.Index.Add` (design doc 26/27) was already designed to be
  called repeatedly against the same graph - `TestIndex_IncrementalAddFindsNewlyAddedVectors`
  already proved this works. Combining the two corpora's embeddings is
  just calling `Add` twice, once per `embeddings.File`.

Neither package needed a single line changed. This is what "prove the
mechanism, defer the wiring" (the split used throughout the whole
semantic-indexing and lexical-index-upgrade components) was for - the
wiring was cheap precisely because the mechanism had already been built
and tested in isolation, ahead of having two real corpora to combine.

## The join key made this safe without needing index alignment

Results are matched back to `Docs`/`Metas` for rendering purely by
stable `Passage.ID()` (via `SearchDeps.ReverseID`), not by array
position - a design already established when hybrid search was first
wired in (design doc 29). That meant the two corpora's `Docs`/`Metas`
slices could simply be concatenated, and `MultiSegment`'s own internal
global-ID numbering (based on segment order) never needs to agree with
that concatenation order at all. The two ID spaces - `MultiSegment`'s
internal `DocID`s, and the array position `ReverseID` maps into - are
completely decoupled, joined only through the passage's own stable
content hash. Positional coupling would have been a real source of
subtle bugs (get the concatenation order wrong once, and results render
against the wrong text) that this design doesn't have to guard against
at all.

## One asymmetry, made explicit rather than papered over

The synthetic corpus's lexical segment is still rebuilt fresh into a
temp directory on every server startup (as it always was) - it's small
and cheap to rebuild. The real corpus's segment
(`data/real/real.seg`) is opened directly, as the precomputed, committed
artifact `cmd/crawl` already produced. Different corpora, deliberately
handled differently, rather than forcing both through the same code
path for uniformity's own sake.

## Real proof

`TestLoadRealCrawlJSONL_MatchesTheCommittedSegment` proves the
correctness property the whole design leans on: reconstructing
`extract.Passage` from `data/real/passages.jsonl` reproduces the exact
`Passage.ID()`s already stored in `data/real/real.seg` - if this didn't
hold, `ReverseID` lookups would silently never match, and every real
search result would fail to render. Beyond that, the real server was
run live: startup logged "serving 30 synthetic + 82 real passages," a
live query for "sunscreen" returned results interleaved from both
corpora in one ranked list, and a live query for "truffle" (the
Unicode-normalization case from design doc 34) correctly surfaced the
real, decorative-Unicode-styled passage as the top result through the
full API - not just in an isolated unit test.
