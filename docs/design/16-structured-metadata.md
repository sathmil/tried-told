# Design Decision 16: Structured Metadata Extraction

Status: decided, Extraction component (in progress)
Date: 2026-07-29

## Decision

Two different sourcing mechanisms for two different kinds of fields:

- **Source-structured fields** (`Product`, `ProductCategory`, `SkinTone`,
  `Climate`): the site itself explicitly marks this up (a `data-*`
  attribute, a labeled form field) - extracted the same way `Product`
  already was, via CSS selectors in the site-specific extractor.
- **Free-text fields** (`DurationOfUse` implemented now; `concern/goal` and
  `positive/negative observations` explicitly deferred): mentioned in
  review prose, not a structured field on most sites - requires pattern
  extraction from the text itself.

## Why free-text extraction is rule-based, not inferred

The project explicitly prohibits inventing missing metadata or using an
LLM to guess. That rules out inference for free-text fields, leaving
deterministic pattern matching (regex, keyword lists) as the only option -
fully explainable (always able to point at the literal matched text), but
**deliberately low recall by design**: `internal/metadata.ExtractDuration`
matches `"3 weeks"` but not `"three weeks"` or `"a while"` - different
phrasings of the same fact that a narrow, honest pattern won't catch.
That's not a bug to fix later; it's the direct, accepted cost of refusing
to guess. Better to leave a field empty than populate it wrong.

## Scope: duration now, concern/sentiment deferred

Duration was judged well-constrained enough to implement now (a
number + a time unit is a genuinely narrow pattern). Concern/goal and
positive/negative observations were deferred - not because they're less
useful, but because there's no real corpus data yet to validate any
keyword list's recall against. Building an elaborate "concern vocabulary"
(acne, dryness, oiliness, sensitivity...) right now would just be guessing
at our own imagined phrasings, with nothing real to check it against -
same reasoning that deferred the SimHash threshold tuning and the generic
boilerplate-removal fallback. Revisit once real judged examples exist.

## Architecture note: a documented seam, not a premature abstraction

Duration extraction is currently invoked directly inside
`ExampleSiteExtractor.Extract`, since it's the only extractor that exists.
Once a second site-specific extractor is written, this should move to a
shared post-extraction enrichment step so every future extractor gets
free-text metadata automatically, rather than each one needing to
remember to wire it in - noted explicitly rather than built speculatively
for an extractor that doesn't exist yet.

## Test worth calling out

The fixture includes both a digit-based duration ("2 weeks", matched) and
a spelled-out one ("two weeks", deliberately not matched) in the same
file, so the test proves both the positive extraction path and the
intentional recall boundary end-to-end, not just in isolated unit tests
for `ExtractDuration` alone.
