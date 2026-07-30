# Design Decision 30: Tombstone-Awareness for Semantic Search

Status: decided, Semantic indexing (complete)
Date: 2026-07-30

## Decision

`internal/semantic.FilterDeleted(results []string, deleted Deleter) []string`
mirrors `bm25.FilterDeleted`, but simpler: `Index.Search` already returns
stable Passage IDs directly, so there's no local DocID to resolve first.
`Index.SearchLive(query, k, deleted)` wraps it with over-fetching:
it asks the HNSW graph for `k * overFetchMultiplier` (3x) candidates,
filters tombstones out, and returns at most k of what remains.

## Why semantic tombstones are a harder problem than BM25's

`bm25.Search` returns every matching document - filtering out deleted
ones afterward just prunes an already-exhaustive list, so recall is
unaffected. `semantic.Index.Search(query, k)` returns an *approximate
top-k* - filtering that result post hoc can silently return fewer than k
usable results, even when plenty of live, relevant passages exist
further out in the graph that Search never had a reason to surface.

## Why over-fetching, not graph-native deletion

Three options existed: filter the plain top-k result (simplest, but
under-returns whenever tombstones land in the top-k); over-fetch a wider
candidate set before filtering (narrows the under-return problem without
touching Search or the graph); or delete tombstoned nodes from the HNSW
graph directly so Search never sees them (closes the gap completely, but
reopens the `coder/hnsw` deletion bug from docs/design/26-hnsw-index.md -
deleting the sole remaining node corrupts the graph - for routine,
ongoing use rather than the one-off "replace on re-add" case already
handled, plus it needs a real sync mechanism keeping the index current
with the deletion log over time, not just a query-time filter).

Chose over-fetching: simpler than touching the graph's mutation surface,
which already has one known bug worth not exercising more than necessary.
The `3x` multiplier is a heuristic starting point, not a literature-backed
constant like BM25's k1/b or RRF's k=60 - worth revisiting against real
deletion rates once there's a real crawled corpus with real churn.

## What over-fetching does and doesn't fix

`TestSearchLive_OverFetchCompensatesForDeletedTopResults` proves the real
case: the two nearest neighbors to a query are both tombstoned, and a
naive `Search(query, 2)` + filter would return zero results, but
`SearchLive` reaches past them into the next candidates and still
returns 2. `TestSearchLive_ReturnsFewerThanKWhenNotEnoughLiveResultsExist`
proves the honest limit of this approach: if too much of the
over-fetched candidate set is also deleted, `SearchLive` returns fewer
than k rather than padding or fabricating results - over-fetching narrows
the under-return gap, it does not close it.

## Deliberately out of scope

Wiring `SearchLive` into the live search API - `cmd/server`'s demo
corpus is static synthetic data with no `crawlstate.DeletionLog` at all,
the same reason `bm25.FilterDeleted` was never wired into the live API
either. Both tombstone-awareness capabilities are proven and ready at the
package level; using them for real requires a real crawled corpus with
real deletions, which is still gated on real-source vetting.
