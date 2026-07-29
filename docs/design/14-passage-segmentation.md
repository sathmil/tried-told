# Design Decision 14: Passage Segmentation

Status: decided, Extraction component (in progress)
Date: 2026-07-29

## Decision

Split extracted review text at **paragraph boundaries**
(`internal/segment.Paragraphs`), not fixed-size token windows and not
"whole extracted unit, never split further."

## The tension

Keeping a whole review as one passage risks diluting relevance: a long,
multi-topic review indexed as one unit means a query's relevant terms
compete with everything else in it for the same document-length budget.
Splitting too aggressively (sentence-level) breaks coherence: "This
sunscreen was great. It didn't leave a white cast..." split in two loses
"it"'s antecedent in the second piece.

## Why paragraph boundaries, not fixed windows

Paragraph breaks are a structural signal the *author* already put there -
no arbitrary mid-sentence cuts, and no window-size parameter to guess at
without real data to tune it against. Crucially, this is a **no-op** for
anything that's already one paragraph, which is most of what this corpus
is expected to look like - it only changes behavior for the rare long,
multi-paragraph review, exactly where the dilution problem would actually
bite.

## Resolving the "more info" concern without sacrificing precision

Initial instinct was "don't segment at all, I want more info" - but that
conflates two different things: retrieval precision (indexing granularity)
and reading richness (display richness). They don't have to be the same
decision. Indexing at finer granularity for scoring precision doesn't lose
anything for the user, since the full original text (extracted, and the
raw fetched page in the content log) remains available to show/link back
to at serving time regardless of what granularity was indexed.

## Real bug caught before this could even work: `.Text()` loses paragraph
## boundaries entirely

Verified directly (didn't assume) that goquery's `.Text()` on a container
with multiple `<p>` children concatenates their contents with **no
separator at all**: `<p>First.</p><p>Second.</p>` produced
`"First.Second."`. Paragraph-boundary segmentation is meaningless if the
boundaries are already gone by the time text reaches it - and worse, this
would silently merge words together across a real paragraph break (e.g.
`"...was high"` + `"The quality..."` -> the single garbled token
`"highthe"` once tokenized), a genuine extraction-correctness bug, not just
a segmentation design gap.

**Fix:** `reviewText` in `internal/extract/example_site.go` finds each
`<p>` child explicitly and joins them with `"\n\n"`, preserving the
boundary as an actual blank line before `segment.Paragraphs` ever sees the
text. Falls back to plain `.Text()` for markup with no nested `<p>` at all.

## Test worth calling out

The extractor's test fixture now includes a genuine multi-paragraph review,
and asserts both that it correctly produces two separate passages *and*
that no passage contains the merged-word failure mode
(`"lightweight.Then"`) that the unfixed version would have produced - a
regression test for the specific bug just found, not just a feature test.
