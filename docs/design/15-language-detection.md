# Design Decision 15: Language Detection

Status: decided, Extraction component (in progress)
Date: 2026-07-29

## Decision

`internal/language.Detect` wraps `lingua-go`, not a hand-rolled detector -
same reasoning as `grobotstxt`/`goquery`: statistical language
classification is a well-studied problem where a trained classifier beats
anything hand-rolled, and it isn't a core algorithmic objective of this
project the way BM25 is.

## Why this matters here specifically

Not just general best practice: this project's pipeline (tokenizer, BM25,
IDF) currently assumes roughly English-like text. Mixed-language content in
one index without tagging pollutes BM25 statistics (a non-English passage
could spuriously share tokens - brand names, numbers - with an English
query, and its presence still skews IDF for everyone). It also turns "the
corpus is probably mostly English" from an assumption into a measurable
fact, which matters directly for the representation-gaps documentation this
project already commits to.

## Library choice: verified, not assumed

Checked directly (license, real API signatures via `go doc` against the
actually-installed package rather than trusting scraped docs) before
committing: Apache-2.0, supports 75 languages, and specifically claims and
tests accuracy on short text - single words and phrases - which matters
here since passages are typically short reviews, unlike the long documents
most language-ID libraries are tuned for.

## A real limitation found by testing, not assumed

`DetectLanguageOf` returns `(Language, bool)`, with `bool` explicitly
meaning "confidently detected." Tested `"xyz"` (3 characters) expecting
`(Unknown, false)` - got `(German, true)` instead. The library's confidence
check guards against *statistical ambiguity between candidate languages*,
not against text simply being too short to carry any signal at all.

**Fix:** `Detect` imposes its own minimum length gate (`MinChars = 20`
runes) before ever calling the detector, on top of trusting the library's
own `ok` result. Never invents a language for text that isn't long enough
to trust - same "don't invent missing metadata" principle already applied
to every other field extraction touches.

## Measured cost: eager model loading

First call to `Detect` in a process takes ~7s (loading all 75 language
models); every call afterward is fast, since the detector is built once at
package scope and reused. Worth knowing as a real, measured startup cost
for whatever eventually calls this in a batch pipeline - not a per-passage
cost.

## Collateral fix: a flaky test exposed by adding a slow package

Adding this package's ~35s model-loading test changed the full suite's
parallel-execution profile enough to expose
`TestCrawl_RespectsPolitenessAcrossConcurrentWorkers` (in `internal/fetch`)
as flaky under `-race` + full-suite CPU contention - it passed reliably in
isolation every time, confirming the underlying politeness logic was never
wrong, just that the test's timing margin (50ms delay, 10ms tolerance) was
too tight for legitimate scheduling jitter under load. Widened to 100ms
delay / 30ms tolerance so the test is robust to real contention rather than
just a lucky run - a genuine hardening fix, not a workaround to hide a real
bug.
