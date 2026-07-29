# Design Decision 13: Near-Duplicate Detection (SimHash)

Status: decided, Extraction component (in progress)
Date: 2026-07-29

## Decision

SimHash (`internal/simhash`), not MinHash, for near-duplicate passage
detection - a 64-bit fingerprint per passage, compared by Hamming distance.

## Why SimHash over MinHash

Both solve the same underlying problem (exact hashing can't catch two texts
that are 95% the same, since a single differing character produces a
completely different hash) but estimate different similarity notions:
MinHash estimates **Jaccard/set-overlap** similarity (how many shingles two
documents share) - a natural fit for near-*verbatim* copies. SimHash
estimates **cosine/weighted-feature** similarity over a compact bit
fingerprint - more tolerant of word reordering and light rewording.

**The deciding factor:** the near-duplicates expected in this corpus are
reworded/paraphrased reposts of the same underlying review (e.g. cross-
posted to a different community with different phrasing), not verbatim
copies. Verbatim copies (page-caching artifacts, syndication) are already
caught for free by the exact-hash dedup registry built earlier - that's not
this layer's job. SimHash's job is specifically the case exact hashing
structurally cannot touch: same opinion, different wording.

## Feature representation

Reused the existing `tokenize.Tokenize` (unigram tokens) rather than
introducing a separate shingling step - each token contributes once per
occurrence as a weighted feature, so a token appearing 3 times in a passage
has 3x the influence on the resulting fingerprint versus one appearing
once.

## Threshold: conservative, and explicitly provisional

Chose conservative (a *small* Hamming-distance threshold, few bits out of
64) specifically because of the minority-erasure risk: content from
underserved contexts (deep skin tones, South Asian undertones) is scarcer
by definition, and scarcer content sharing a smaller, more specific
vocabulary looks *more* similar to itself by pure overlap - a loose
threshold would disproportionately thin out exactly the perspectives this
project exists to surface, since majority-pattern content has plenty of
redundant copies to spare while minority content doesn't.

**The exact numeric threshold is deliberately not locked in from theory
alone** - it needs validation against real judged near-duplicate examples
once real corpus data exists, per the project's "no quality claim without
a reproducible measurement" rule. `IsNearDuplicate` takes `maxDistance` as
a parameter rather than a hardcoded constant for exactly this reason.

## Test worth calling out

`TestFingerprint_ParaphrasedTextIsCloserThanUnrelatedText` doesn't assert a
specific distance value (which would be a brittle, hard-to-justify magic
number) - it asserts the *comparative* property that actually matters: a
paraphrase of the same review must land closer than genuinely unrelated
text. That's a real empirical check that SimHash is behaving as claimed on
data resembling this project's actual corpus, not just a theoretical
assumption about the algorithm.
