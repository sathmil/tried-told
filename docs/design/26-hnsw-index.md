# Design Decision 26: HNSW Semantic Index

Status: decided, Semantic indexing (in progress)
Date: 2026-07-30

## Decision

`internal/semantic.Index` wraps `github.com/coder/hnsw`'s `SavedGraph[string]`,
keyed directly by `extract.Passage.ID()` (no separate local-ID mapping
needed - unlike the lexical index's postings, HNSW nodes aren't
delta-compressed, so there's no reason to avoid the 64-char string as the
key). Distance is `hnsw.CosineDistance`, matching bge-small-en-v1.5's
normalized-embedding convention (design doc 25).

## Two real library bugs, found by testing instead of trusting the docs

`coder/hnsw`'s own documentation for `Add` states: *"If another node with
the same ID exists, it is replaced."* Verified directly (same practice as
every dependency this session) and found **false** for v0.6.1:

1. **`Add` panics on a duplicate key** (`"node not added"`) rather than
   replacing it - reproduced with both a 20-node graph and a 1-node graph,
   ruling out a small-graph-only explanation.
2. **The documented workaround (`Delete` then `Add`) has its own bug**:
   deleting the *only* remaining node leaves the graph in a state where
   the next `Add` panics with a nil pointer dereference
   (`assertDims` dereferencing something that isn't initialized for an
   emptied-via-Delete graph). `Delete` then `Add` works correctly when
   *other* nodes remain - isolated by testing both cases directly, not
   assumed from the first failure alone.

**Fix**: `Index.Add` checks whether a key already exists via `Lookup`
first. If the graph has more than one node, `Delete` then re-`Add` (the
documented-but-broken path, which works fine once more than one node is
present). If the graph would become empty, discard it and assign a fresh
`hnsw.NewGraph[string]()` instead of deleting in place - verified
empirically to sidestep the corrupted-empty-graph state entirely.

This is documented as a real, upstream library limitation, not hidden
inside a silent workaround - if a future version of `coder/hnsw` fixes
either bug, this workaround becomes unnecessary rather than incorrect, and
the comment explains why it exists.

## Real semantic-relevance proof, not just mechanical correctness

`TestIndex_FindsSemanticallyRelevantPassageWithRealEmbeddings` uses real
BGE embeddings for three deliberately unrelated passages (sunscreen,
pizza, car maintenance) and a real query embedding (with the BGE query
instruction prefix applied, unlike the passage embeddings - the asymmetry
from design doc 25 matters here too) - and confirms the sunscreen passage
ranks first for a sunscreen-related query. Fixtures generated once via the
real model and committed, not regenerated at test time.

## Deliberately out of scope for this pass

**How a live user query gets embedded at request time** is a genuinely
open architectural question, not solved here: the query needs the same
neural network that generated passage embeddings, which is a PyTorch
artifact, while the stack mandate says Go owns the query service. Real
options for a later pass: a small persistent Python embedding
microservice the Go query service calls internally (keeps "Go orchestrates
serving" mostly intact), or exporting the model to ONNX and using Go ONNX
bindings for a fully Go query-time path (more upfront work, more faithful
to "Go owns serving"). Not decided yet - this pass only needed a way to
prove the *index* works correctly, which the committed real query fixture
provided without needing to solve that question first.

Also still open: wiring this into hybrid ranking (RRF against BM25),
tombstone-awareness (matching `bm25.FilterDeleted`), and connecting it to
the real extraction pipeline rather than test fixtures - all separate,
later pieces.
