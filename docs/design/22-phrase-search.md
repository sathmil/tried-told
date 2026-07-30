# Design Decision 22: Phrase Search

Status: decided, Lexical index upgrade (in progress)
Date: 2026-07-29

## Decision

`Segment.PhraseSearch(phrase string) []int` (`internal/diskindex/phrase.go`):
returns local DocIDs where the phrase's words occur as an exact,
consecutive, in-order sequence - not just documents containing all the
words somewhere.

## Why BM25 never needed positions, and why phrase search does

BM25 is a bag-of-words model: its formula only depends on how many times a
term occurs (`f(t,D)`) and how many documents contain it (`df(t)`) - never
*where*. Two documents with identical term-frequency profiles score
identically regardless of word order, which is exactly why Milestone 1's
index worked correctly with `Posting{DocID, Freq}` and no positions at
all. Phrase search asks a fundamentally different question - not "does
this document discuss these words" but "do these words appear in this
exact order, adjacent to each other" - which requires knowing exact token
offsets.

## Algorithm

For an N-word phrase: fetch postings for every word, intersect the
document sets (candidates must contain *all* words - a document missing
even one word can't possibly contain the phrase), then for each candidate
check whether some starting position `p` has word `i` at `p+i` for every
`i` in `[0, N)`.

**Query optimization, not just correctness**: candidate generation starts
from the rarest word (fewest postings), not an arbitrary term - the same
principle real query planners use for boolean AND, minimizing how many
documents get checked against the remaining terms.

**Position-containment check uses binary search**
(`sort.SearchInts`), not linear scan - positions within one posting are
already sorted (maintained since design doc 01's sorted-postings
invariant), so this is a real, not incidental, complexity improvement.

## Scope boundary: segments only, not the in-memory index

Deliberately built as a method on `diskindex.Segment`, not folded into the
shared `bm25.Index` interface - the in-memory Milestone 1 index never
stored positions at all (by design, since BM25 never needed them), so
there's nothing for an in-memory equivalent to operate on. `bm25.Posting`
stays position-free on purpose (entry 22 of `docs/LEARNING.md`); phrase
search is an honest capability gap between the two backends, not something
papered over with a fake/empty implementation for the backend that can't
support it.

## Test worth calling out

`TestPhraseSearch_RequiresAdjacentInOrderWords` is the actual proof
positions do real work: a document containing "cast" and "white" - just
not adjacent, and in reverse order - must NOT match "white cast", while a
document with them genuinely consecutive must. Without this test, a buggy
implementation that silently degraded to a plain AND-query (ignoring
positions entirely) would still pass every other test.

## Deliberately not built yet

Snippet generation (showing a highlighted excerpt around a match) is the
other capability positions enable, per the project spec, but wasn't
requested in this pass and isn't built - a natural next piece, not bundled
in here.
