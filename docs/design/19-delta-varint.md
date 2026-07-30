# Design Decision 19: Delta + Variable-Byte Encoding

Status: decided, Lexical index upgrade (in progress)
Date: 2026-07-29

## Decision

`internal/postings.EncodeDeltas`/`DecodeDeltas`: delta-encode a strictly
increasing sequence (gaps between consecutive values, exploiting the
sortedness the index already guarantees), then varint-encode each gap
using `encoding/binary`'s standard `PutUvarint`/`Uvarint` (LEB128-style:
7 payload bits + 1 continuation bit per byte) rather than hand-rolling
byte-packing ourselves - the varint *mechanism* is generic tooling (same
reasoning as using `math/bits.OnesCount64` in the Bloom filter); the delta
*logic* is the actual IR technique we're here to build.

## Why this pays off, concretely

Applies at two levels with the identical technique: DocID gaps between
postings for one term, and position gaps within one posting's position
list (positions are also naturally increasing - a document is scanned
left to right). Measured, not assumed: a realistic dense posting list
(1,000 DocIDs, gap of 3 throughout - representative of a fairly common
term) compresses to **12% of naive 8-byte fixed-width encoding**.

## Why postings had to be sorted in the first place - the payoff shows up twice now

Design doc 01 sorted postings by DocID for merge-join AND-query
performance. This encoding is the *second* reason that decision mattered:
delta encoding only produces small numbers (and therefore small varints)
because the sequence is sorted and strictly increasing. An unsorted
postings list would still "work" but would lose essentially all the
compression benefit - deltas between random DocIDs are just as large as
the DocIDs themselves.

## Correctness guard: panic on non-increasing input

Casting a negative `int` to `uint64` (which happens if the input isn't
strictly increasing - a decreasing or repeated value produces a
non-positive delta) silently wraps around to a huge, garbage value rather
than failing - the exact same category of invisible-corruption risk as
the URL-normalization `%2F` bug. Checked explicitly and panics rather than
trusted, since every real caller (DocIDs are unique per BuildIndex's own
invariant; positions are unique since each token occupies one distinct
position) is expected to already satisfy this.

## Integrity: corruption returns an error, never a panic or silent garbage

`DecodeDeltas` on a truncated/corrupt varint (e.g. from disk corruption)
returns an error - this is the "integrity checks and corruption tests"
requirement made concrete, not deferred: `TestDecodeDeltas_CorruptionReturnsErrorNotPanic`
feeds it a deliberately truncated varint byte and confirms it fails
loudly with an error rather than panicking the whole process or silently
producing wrong data.

## Test worth calling out

`TestEncodeDeltas_CompressesRealisticPostingList` doesn't assert a
theoretical compression ratio - it encodes a realistic posting list and
measures the actual byte counts, logging the real percentage (12%). A
reproducible measurement, per the project's own rule, not a claim taken
on faith from the algorithm's reputation.
