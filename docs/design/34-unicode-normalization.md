# Design Decision 34: Unicode Normalization in the Tokenizer

Status: decided, Real-source sourcing (in progress)
Date: 2026-07-30

## Decision

`tokenize.TokenizeWithOffsets` runs each token's text through
`golang.org/x/text/unicode/norm`'s NFKC (compatibility normalization)
before lowercasing it, so decorative Unicode variants of ordinary Latin
letters - e.g. Mathematical Bold "𝐓𝐫𝐮𝐟𝐟𝐥𝐞" - fold to the same term as
plain "Truffle". This only changes `Token.Text`; `Start`/`End` still
index into the original, un-normalized source text exactly as before.

## The gap, and why it's a real gap

`unicode.IsLetter` already classified Mathematical Bold characters as
letters, so `Tokenize` was already including them in tokens - it just
never folded them to the same term as their plain-ASCII equivalent,
because they're genuinely different Unicode codepoints (U+1D413
MATHEMATICAL BOLD CAPITAL T vs. U+0054 LATIN CAPITAL LETTER T), not a
font rendering difference. `unicode.ToLower` only handles case-pair
folding, not this. A real crawled post (docs/design/33-real-crawl.md)
used this styling for emphasis, meaning a plain-text query for "truffle"
would never match content a human reader sees as saying exactly that -
a real, silent recall failure the synthetic corpus could never have
surfaced, since nobody hand-writing test fixtures thinks to add
decorative Unicode styling.

## Why NFKC, not NFC/NFD

Unicode defines canonical equivalence (NFC/NFD - the same abstract
character represented as one composed codepoint vs. a base character
plus combining marks, e.g. "é" as U+00E9 vs. "e" + combining acute) and
compatibility equivalence (NFKC/NFKD - characters that are the same
*text* for most purposes but carry formatting distinctions: Mathematical
Bold, full-width forms, ligatures, superscripts). Mathematical Bold
letters are a compatibility relationship, not a canonical one - NFC
leaves them untouched. NFKC's compatibility decomposition step maps
them to their plain form, then its canonical composition step re-merges
any genuinely composed characters (e.g. accented Latin letters stay
composed, not decomposed into base+diacritic), so already-well-formed
text like "café" round-trips unchanged - verified directly via
`TestTokenizeWithOffsets_MultiByteRuneOffsetsAreByteNotRuneIndexed`,
which still passes unmodified.

## Where normalization happens, and why it can't happen upstream

Two places were possible: normalize the whole input text before
tokenizing, or normalize only each token's `Text` once extracted.
Normalizing the whole text first is simpler, but NFKC can change byte
length (a 4-byte Mathematical Bold letter becomes a 1-byte plain one),
so every downstream byte offset would shift - breaking
`TokenizeWithOffsets`'s contract that `Start`/`End` index into the
*original* text (see docs/design/32-snippets.md), unless the original
text itself were replaced by its normalized form everywhere, including
what a snippet shows a user. That would mean silently rewriting a
blogger's actual styled text into plain ASCII in search results - not
acceptable, since the point of snippets is showing what was actually
written.

Chosen instead: normalize only the token text built up in
`TokenizeWithOffsets`'s `flush` step, leaving `Start`/`End` untouched.
`Token.Text` (what gets matched and indexed) and the original text a
snippet slices from (what gets displayed) are deliberately two different
representations of the same content - the same "encode what's needed for
matching separately from what's needed for display" pattern already used
for the BGE query-instruction prefix (design doc 25) and RRF's rank vs.
score (design doc 28).

## Real proof, including closing the loop

`TestTokenize_FoldsDecorativeUnicodeVariantsToPlainASCII` proves the
folding itself. `TestTokenizeWithOffsets_NormalizationDoesNotAlterOriginalTextOffsets`
proves the design choice directly: after normalization, `Start`/`End`
still recover the *original* styled substring, not the normalized one.
`TestWordPressBlogExtractor_RealDecorativeUnicodeIsSearchable` closes
the loop against the exact real content that surfaced the gap in the
first place, not a synthetic stand-in. `data/real/real.seg` (a build
artifact, reproducible from `data/real/passages.jsonl`) was rebuilt
with the fixed tokenizer and re-verified live: a real BM25 query for
"truffle" went from 0 hits to 1, against the real segment.

## Deliberately out of scope

Other Unicode normalization concerns - grapheme cluster segmentation,
locale-specific case folding, script-mixing detection - are not
addressed here. This pass fixes the one concrete, real gap actually
found (compatibility-variant Latin letters), not a general-purpose
internationalization overhaul.
