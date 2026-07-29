# Design Decision 02: Tokenization & Normalization

Status: decided, Milestone 1
Date: 2026-07-27

## Decision

A token is a maximal run of consecutive runes that are **all letters**, or a
maximal run of consecutive runes that are **all digits**. Any other rune
(whitespace, punctuation, hyphens, `%`, etc.) is a boundary and is dropped. A
transition from a letter-run to a digit-run (or vice versa) is also a boundary,
even with no separating character.

```
"SPF50"              -> ["SPF", "50"]
"SPF 30"             -> ["SPF", "30"]
"non-comedogenic"    -> ["non", "comedogenic"]
"10% niacinamide"    -> ["10", "niacinamide"]
"doesn't"            -> ["doesn", "t"]
```

Every token is lowercased before being used as an index term.

No stopword removal in Milestone 1. No stemming in Milestone 1.

## Rationale

- **Letter/digit class boundary, not just alnum/non-alnum.** This corpus writes
  the same fact both ways ("SPF50" vs "SPF 30", "2%" vs "2 %"). If letter-runs and
  digit-runs were one combined "word" class, `"SPF50"` and `"SPF 30"` would
  tokenize differently and a query for `"SPF"` would only match one of them —
  recall loss driven purely by a reviewer's typing habits, not by relevance.
- **Hyphens need no special rule.** Under rune classification, a hyphen is
  neither a letter nor a digit, so it's already a boundary. No prefix-stripping
  logic is needed for `"non-comedogenic"` — it falls out for free as
  `["non", "comedogenic"]`.
- **No stopword removal.** At 500-1,000 documents, postings-list bloat for common
  words is not a real cost, and BM25's IDF term already down-weights terms that
  appear in nearly every document, so common words mostly self-suppress in
  ranking without explicit removal. This is primarily an index-size optimization
  that becomes worth revisiting at the 50k-100k+ corpus scale, not a correctness
  requirement now.
- **No stemming.** Deferred as future work — it introduces its own design
  decision (which stemmer, how aggressive) not needed for the simplest complete
  vertical slice.

## Alternatives considered

- **Whitespace split + regex punctuation stripping.** Rejected: doesn't cleanly
  handle punctuation glued to words (`"cast."`, `"(oily)"`) and isn't
  Unicode-aware for any non-ASCII text.
- **Boundary only at non-alphanumeric characters** (letters and digits treated
  as one class). Rejected: see SPF50/SPF 30 inconsistency above.
- **Stripping negation prefixes/morphology (e.g. dropping "non-").** Rejected:
  actively destroys the negation signal that the no-stopword-removal decision was
  specifically trying to preserve.

## Known limitation

Bag-of-words term overlap cannot, by itself, distinguish `"non-comedogenic"`
from `"comedogenic"` — a document mentioning either contains the token
`"comedogenic"`. The real fix is position-aware / phrase retrieval, planned for
the lexical indexing milestone (word positions in postings).
