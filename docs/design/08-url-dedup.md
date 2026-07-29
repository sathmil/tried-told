# Design Decision 08: URL Dedup Registry + Bloom Filter

Status: decided, Crawler component (in progress)
Date: 2026-07-29

## Decision

Two-layer dedup (`internal/dedup`):

1. **Bloom filter** (`bloomFilter`, bit-packed `[]uint64`): fast first-pass
   check. `MightContain` never false-negatives - if it says an item is new,
   that's always correct. It can false-positive - it may say "maybe seen"
   for something never actually added.
2. **Exact registry** (`Registry.exact`, `map[urlKey]struct{}`): the
   authoritative source of truth, consulted only when the Bloom filter says
   "maybe."

`SeenOrAdd(normalizedURL)` is a single check-and-insert call: Bloom "no" is
trusted immediately (definitely new, add to both layers); Bloom "maybe"
falls through to the exact map, which gives the real answer.

## Why the Bloom filter can't be the only registry

A false positive means "probably seen" for a URL that was never actually
crawled. Trusting that verdict alone would mean silently and permanently
losing real content - no error, nothing to notice later. The opposite
failure (redundantly recrawling something already seen) is just wasteful,
not catastrophic. So the Bloom filter can only ever be a fast filter that's
allowed to say "maybe" - never the final word.

## Sizing

Standard formulas: `m = -(n·ln p)/(ln 2)²` bits, `k = (m/n)·ln 2` hash
functions. For `n=1,000,000` expected URLs (250k+ doc scale, since most
discovered links never become indexed docs) at `p=1%`: `m ≈ 9.6M bits ≈
1.2 MB`, `k ≈ 7`. That's the entire value proposition - a ~1.2 MB structure
resolves most "have I seen this" checks instantly, and only the rarer
"maybe" answers pay the cost of touching the larger exact registry.

`k` hash functions are simulated from two real ones (`hash/fnv`, double-
hashed with a seed prefix) via Kirsch-Mitzenmacher (`g_i(x) = h1(x) +
i·h2(x)`), avoiding the need for `k` genuinely independent hash functions.

## Exact registry stores hashes, not full URLs

`urlKey` is a SHA-256 digest of the normalized URL, truncated to 128 bits.
The tradeoff being accepted isn't "URLs aren't stored" - it's **hash
collision**: two different URLs could in principle map to the same key,
which is the same false-positive failure mode as the Bloom filter, just
recurring one layer deeper, at the layer meant to be authoritative. Accepted
because a 128-bit hash's collision probability at this project's realistic
scale (thousands to low millions of URLs, nowhere near the ~2^64-item
birthday-bound territory) is astronomically small, in exchange for storing
16 bytes per URL instead of 50-150+.

## Implementation note: true bit-packing

The Bloom filter uses `[]uint64` with manual bit shifting/masking, not
`[]bool`. A `[]bool` in Go takes 1 byte per element - 8x the memory of a
real bit array - which would undercut the entire memory argument for using
a Bloom filter in the first place.

## Scope boundary

Persistence (surviving a restart) is explicitly out of scope here - both
layers are in-memory for now. That's a separate, later decision (frontier/
storage persistence task), not bundled into this one.

## Test worth calling out

`TestRegistry_ExactLayerOverridesBloomFalsePositive` deliberately constructs
a Bloom filter with `m=1` bit (guaranteeing every check reports "maybe") and
confirms the exact registry still returns the objectively correct answer -
a direct, reproducible test of the reason this design has two layers at
all, not just an add-then-check smoke test.
