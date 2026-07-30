# Design Decision 25: Offline Embedding Generation + Binary Interchange Format

Status: decided, Semantic indexing (in progress)
Date: 2026-07-30

## Decision

**Model**: `BAAI/bge-small-en-v1.5` (MIT license, 384-dim, retrieval-tuned -
verified via its model card rather than assumed). **Generation**: Python
(`python/generate_embeddings.py`, `sentence-transformers`), scoped
strictly to offline embedding generation per the project's original stack
mandate ("Python allowed for offline embedding generation... Go for the
query service"). **Interchange**: a precisely documented binary format
(TTE1), read by `internal/embeddings` - the same header/sections/checksum
approach as the lexical index's segment format, for consistency.

## Why a retrieval-tuned model, not a general-purpose one

General-purpose sentence-similarity models (e.g. `all-MiniLM-L6-v2`) are
trained on paraphrase/NLI/STS data - "are these two sentences saying the
same thing." Retrieval-tuned models (BGE, E5) are trained with a
contrastive objective specifically for "given a query, find the right
passage" - an asymmetric task, not a symmetric similarity one. Since our
actual task is retrieval, not paraphrase detection, the retrieval-tuned
category fits better regardless of size.

## Asymmetric encoding: queries and passages are not treated the same

BGE models require an instruction prefix on queries only
("Represent this sentence for searching relevant passages: "), never on
passages. `generate_embeddings.py` embeds passages and deliberately adds no
prefix; encoding a query correctly (with the prefix) is the query
service's responsibility at search time, not this script's - noted
explicitly so the asymmetry isn't lost between the two call sites.

## Format design: fixed header + offset/length sections + trailing checksum

Same pattern as `internal/diskindex`'s segment format: `Magic`, `Dim`,
`Count`, `GeneratedAtUnix`, then offset/length pairs for a `Version`
string section and a `Records` section (each record: 64-byte
`Passage.ID()` + `Dim` little-endian float32s), trailer CRC32 covering
everything above it.

## The real risk this format carries, and how it was actually verified

Every other binary format in this project has been Go writing and Go
reading - self-consistent even if an assumption about serialization
behavior were wrong. This is the first format where **Python writes and Go
reads** - a genuine cross-language byte-compatibility risk, not just a
theoretical one. Reasoning alone (`encoding/binary.Write`'s field-by-field,
no-padding serialization, matched against Python's `struct.pack("<...")`
standard-size, no-padding, little-endian mode) predicts they'll match
byte-for-byte - but prediction isn't proof. **Verified empirically**: ran
the real Python script against real passages, read the real output file
with the real Go reader, and confirmed correct version string, dimension,
passage IDs, and unit-normalized vectors (L2 norm 1.0, confirming
`normalize_embeddings=True` worked as expected) - not just a hand-crafted
Go-authored test file.

## `VERSION` string: our own convention marker, not the model's HF revision

`"bge-small-en-v1.5-cosine-normalized-v1"` identifies the model **and**
our usage convention (normalization, no query-instruction on passages) in
one string - not the same thing as the model's HuggingFace commit hash.
If either the model or our own convention changes, this string should
change, so a consumer can check "does this file's version match what I
expect" before trusting it - the "model and embedding version tracking"
requirement made concrete.

## Test worth calling out

`TestOpen_RealPythonGeneratedFile` uses a fixture committed once from
genuine `generate_embeddings.py` output (not regenerated at test time,
since that would require Python and the actual model available wherever
tests run) - proof the reader works against real Python output, not just
Go-authored bytes designed to match the reader's own assumptions.

## Deliberately out of scope for this pass

Building and querying the actual HNSW graph from these vectors - this
pass proves the embedding-generation and interchange-format pipeline
works end-to-end; wiring it into `github.com/coder/hnsw` is the next,
separate piece.
