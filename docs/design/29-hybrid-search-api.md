# Design Decision 29: Wiring Hybrid Ranking into the Search API

Status: decided, Semantic indexing (complete)
Date: 2026-07-30

## Decision

`internal/api.SearchHandler` now takes a `SearchDeps` struct instead of a
raw `index.Index`, and ranks results via `hybrid.Fuse` (design doc 28)
when a semantic index and query embedder are configured, falling back to
BM25 alone otherwise.

## Resolving the backend question from design doc 28

The demo corpus (`data/synthetic/experiences.jsonl`) had no stable
`Passage.ID()` at all - `corpus.LoadJSONL` only ever produced
`index.IndexDoc`/`index.DocMeta` for the in-memory `index.Index`. Hybrid
ranking needs a passage-ID join key shared by both rankings, and the
disk-backed segment format (design doc 20) already solved exactly that
(local int ID <-> `Passage.ID()`), so `cmd/server` now:

1. Converts the loaded corpus into `[]extract.Passage` via a new
   `corpus.ToPassages` (a direct field mapping - `Source` stands in for
   `SourceURL`, since the synthetic corpus has no real crawl URL, but
   `Passage.ID()` only needs it to be stable and text-differentiating).
2. Builds a real segment via `diskindex.BuildSegment`, into a temp
   directory rebuilt fresh on every server startup - the corpus is 30
   passages, so there's no reason to persist the segment across restarts.
3. Generates real embeddings for those same 30 passages offline, via the
   existing `python/generate_embeddings.py`, committed at
   `data/embeddings/synthetic.bin` (and the JSONL that produced it, so
   it's regenerable, not just a mystery binary).
4. Builds the HNSW graph from that file at startup via `semantic.Index`,
   also into the same temp directory.

`docs`/`metas` (used for rendering) are left untouched - only the
*search* backend changed, from `index.Index` to the segment, since
`index.IndexDoc`/`DocMeta` still assign IDs in the same corpus-file order
`corpus.ToPassages` does, so `ReverseID[Passage.ID()] == docID` holds by
construction.

## Semantic search is an enrichment, not a hard dependency

If `data/embeddings/synthetic.bin` or the graph build fails, the server
logs it and starts anyway, serving BM25-only search - it doesn't refuse
to start over an optional signal. Symmetrically, if the live
`embed_service.py` call fails *per request* (e.g. the microservice isn't
running), `SearchHandler` logs a warning and falls back to the BM25-only
ranking for that request rather than returning an error - proven by
`TestSearchHandler_FallsBackToBM25OnlyWhenEmbedderFails`, which asserts a
200 with BM25-only results even when the fake embedder always errors.
This is a standard graceful-degradation choice for an optional enrichment
signal, not a new design question: search should degrade in ranking
quality when semantic search is unavailable, not degrade in availability.

## The `score` field is gone; `rank` replaces it

The old `searchResult.Score` was a raw BM25 score. Once results can be
produced by RRF (design doc 28), which explicitly discards score
magnitude in favor of rank position, there's no single number left that
means the same thing across both ranking modes - showing a stale BM25
score on a fused result would misrepresent how it was actually ranked.
`Rank` (the result's 1-indexed position in whichever ranking actually
produced the response) is honest about what's known in both modes.

## Real proof, not just wiring

`TestSearchHandler_FusesLexicalAndSemanticResults` constructs a query
that matches doc 1 lexically only, with a faked embedding that's nearest
to doc 0's - and asserts both docs come back, proving fusion actually
merges the two signals rather than one silently winning. Beyond the unit
tests, the real server was started end-to-end against the real
`embed_service.py` and real corpus embeddings: a query worded without the
corpus's literal "greasy/oily" phrasing still surfaced the correct
moisturizer passages by blending lexical and semantic signal - the
concrete case hybrid ranking exists to solve.

## Deliberately out of scope

- Tombstone-awareness for the semantic side of a hybrid result (BM25
  results already support `FilterDeleted`; semantic results don't yet).
- Wiring this same pattern to the real crawled corpus rather than the
  synthetic demo data - still gated on real-source vetting.
