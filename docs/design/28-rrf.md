# Design Decision 28: Hybrid Ranking via Reciprocal Rank Fusion

Status: decided, Semantic indexing (in progress)
Date: 2026-07-30

## Decision

`internal/hybrid.Fuse` combines BM25's lexical ranking and the HNSW
index's semantic ranking into one fused ranking via Reciprocal Rank
Fusion (RRF): each ranking contributes `1/(k + rank)` to every ID it
contains (rank 1-indexed, `k = 60`, the standard RRF default); an ID's
fused score is the sum across every ranking it appears in.

## Why RRF over a weighted score combination or two-stage retrieval

BM25 scores and cosine-similarity scores are on incomparable scales -
BM25 is unbounded and corpus-dependent, cosine similarity is bounded in
[-1, 1]. A weighted linear combination (`alpha*bm25 + (1-alpha)*cosine`)
needs per-query normalization to make that combination meaningful at all,
and normalization done per-query (e.g. min-max) is fragile: one unusually
high score for a given query skews that query's whole scale, and `alpha`
itself has no principled default.

RRF sidesteps the entire problem by never looking at a score - only at
rank position. That's the actual reason it was chosen: it avoids the
normalization headache by construction, not by tuning it away. The
tradeoff accepted: RRF discards magnitude, so a passage that's a
dramatically better BM25 match than everything else gets credit only for
being ranked #1, not for its margin over #2.

Two-stage retrieval (BM25 top-N re-ranked by semantic similarity) was
also considered and rejected for this pass: its recall ceiling is capped
by whatever the first-stage lexical retrieval already missed, which
defeats the purpose of adding semantic search - i.e. finding passages
that share no vocabulary with the query but mean the same thing.

## The property that actually justifies RRF over "just pick one signal"

`TestFuse_AgreementBeatsASingleSourceTopRank` is the real proof, not just
mechanical correctness: given a passage ranked #2 in *both* the lexical
and semantic rankings, it outranks a passage ranked #1 in only one of
them. Combined, weaker evidence beats strong single-source evidence -
that's the whole reason to fuse two rankings instead of using whichever
one seems better.

## Deliberately kept as a standalone building block, not wired in yet

`hybrid.Fuse` takes `[][]string` (stable passage IDs, best-first) and
returns a fused `[]string` - it knows nothing about `bm25.Result`,
`DocID`s, or `semantic.Index` directly. That mirrors the same separation
already used for `bm25.FilterDeleted` (design doc 24): resolving a
`DocID` to a stable `Passage.ID()` is a distinct step from scoring or
fusing, since not every backend has that mapping (the in-memory
`index.Index` used by `internal/api`'s current demo handler has no
`PassageID` resolver at all - only the disk-backed segments do).

Wiring this into a real search request - calling `embedclient`, running
`semantic.Index.Search`, resolving BM25 `DocID`s to passage IDs via
`bm25.PassageIDResolver`, and fusing the two - is deliberately not done
in this pass. It surfaces a real open question of its own: the demo
`internal/api` handler searches the in-memory synthetic corpus, while the
semantic index and disk segments are keyed by stable passage ID -
reconciling which corpus backend the live search API actually serves is
a separate decision, not a detail to smuggle into this one.
