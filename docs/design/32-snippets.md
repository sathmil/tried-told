# Design Decision 32: Search Result Snippets

Status: decided, Lexical index upgrade (complete)
Date: 2026-07-30

## Decision

`internal/snippet.Extract(text, query string) Snippet` returns a
window of text around the passage's densest cluster of query-term
matches, plus the byte spans of each match within that window for
highlighting. It shares `tokenize.TokenizeWithOffsets` - a new addition
to `internal/tokenize` that records each token's byte span, alongside
the existing `Tokenize` - with the retrieval layer, so "this word
matched the query" means the identical thing for highlighting as it
does for ranking.

## Why the matching logic has to share the retrieval tokenizer

This project has already been burned once by two code paths quietly
disagreeing about the same fact (the URL-normalization `%2F` case) and
once by trusting a library's own claim about its behavior instead of
checking it (`coder/hnsw`'s `Add` documentation, design doc 26). A
snippet highlighter that re-implements "does this text match the query"
with its own substring or whitespace-based heuristic is exactly that
same risk again: retrieval tokenizes `"SPF50"` into `["spf","50"]`, so a
query `"SPF 50"` matches it at the token level - but a literal substring
search for `"SPF 50"` would never find `"SPF50"` in the passage text,
silently failing to highlight a term that genuinely mattered for
ranking. `Extract` avoids this by tokenizing both the query and the
passage text through the exact same function retrieval and indexing
use, guaranteeing the two notions of "match" can never drift apart.

`Tokenize` is now defined in terms of `TokenizeWithOffsets` (a thin
wrapper extracting just the `Text` field) rather than as a second,
parallel implementation - the same reasoning again: if the two ever
diverged, ranking and highlighting could disagree about what a token
is, quietly.

## Picking a window: match density, not first occurrence

`bestWindow` slides a fixed-size (12-token) window across the passage
and scores each position by how many query terms it contains, returning
the highest-scoring position - not simply the window starting at the
first match. A passage can mention a query term once early and then
discuss it in depth later; anchoring on the first occurrence would
often show the least informative part of the passage. Ties favor the
earliest window, for a deterministic result. When no query term appears
in the passage at all (a real case: a passage surfaced purely by
semantic search may share no vocabulary with the query), every window
scores 0, and the tie-break naturally falls back to a window at the
start of the text - a reasonable default excerpt rather than an empty
or arbitrary result.

## An incidental fix while running the full suite

Running the full test suite (a standard step in this project before
declaring anything done) surfaced a real, pre-existing flake in
`TestSearchLive_OverFetchCompensatesForDeletedTopResults` (design doc
30, entry 31), unrelated to this change. Reran it 100 times in
isolation and confirmed it failed about 12% of the time: `coder/hnsw`'s
approximate search can rank two near-tied candidates in the "wrong"
relative order even on an exhaustive, 5-node graph, because of its
own (unseeded) randomized level assignment. Neither raising `EfSearch`
nor padding the graph out to a more realistic size fixed it (padding
actually made the test *worse*, since `SearchLive`'s over-fetch became
selective rather than exhaustive against a bigger graph) - this is
inherent small-scale ANN behavior, not a bug in `SearchLive`. The test
was asserting a stronger claim than `SearchLive` actually makes (an
exact ranking between two specific survivors) instead of the real
guarantee (over-fetching successfully returns 2 live, non-tombstoned
results at all); loosened the assertion to match what's actually
guaranteed, rather than trying to force a tiny approximate graph to
behave deterministically. Verified stable across 30 repeated runs
afterward.

## Deliberately out of scope

Wiring `Extract` into `internal/api.SearchHandler` - the synthetic demo
corpus's passages are already shorter than the default 12-token window,
so there's limited practical value in wiring it into that specific demo
right now; the mechanism is proven and ready for whenever passages are
long enough to need it (real crawled content, still gated on
real-source vetting).
