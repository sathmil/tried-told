# Design Decision 36: Wiring Snippets into the Search API

Status: decided, Lexical index upgrade (complete)
Date: 2026-07-31

## Decision

`SearchHandler` now runs each result's passage text through
`snippet.Extract` and replaces the old full-text `text` field with
`snippet_html` - a single, already-escaped HTML string with the matched
query terms wrapped in `<mark>`. The API response is lean: one rendered
field, not a raw excerpt plus separate offsets for a client to interpret.

## Replacing text, not adding alongside it

The alternative was keeping the full passage text and adding a
`snippet`/`matches` field alongside it. Rejected in favor of replacing:
this is a small demo corpus where the full passage was rarely much
longer than the snippet anyway, and a leaner response - one field
telling the truth about what actually matched - was preferred over
carrying both a fuller and a more focused view of the same content with
no current consumer needing the difference.

## Why the highlight markup is built in Go, not JavaScript

`snippet.Span` offsets are byte offsets into a UTF-8 string - correct
and natural on the Go side. JavaScript strings are indexed in UTF-16
code units, not bytes, and this project's real content (design docs 33,
34) has plenty of characters where that distinction matters: curly
quotes, accented letters, and - critically - the decorative Unicode
"Mathematical Bold" styling, which sits in a Unicode supplementary plane
and requires a UTF-16 *surrogate pair* (2 code units) per character.
Sending raw byte offsets to the browser and reconstructing highlighting
there with `text.slice(start, end)` would silently misalign or corrupt
the highlight around exactly this kind of real content - the same
byte/rune/UTF-16 offset care this project has needed before, now
crossing a language boundary instead of staying inside Go.

The alternative (convert byte offsets to UTF-16 offsets before sending
JSON) would work, but pushes a second offset-translation concern onto
every future consumer of this endpoint, not just today's one static
page. Building the final marked-up HTML in Go instead means no offset
math ever crosses the language boundary at all: Go's own byte-indexed
slicing is already correct, so there's nothing left for JavaScript to
misinterpret. `renderSnippetHTML` HTML-escapes the plain-text runs
(`html.EscapeString`) and wraps only the matched runs in literal
`<mark>...</mark>` - real crawled content is untrusted input, so the
escaping isn't optional, proven by
`TestRenderSnippetHTML_EscapesPlainTextAroundMatches`, which checks a
passage containing a literal `<script>` tag comes back escaped, with
the match still highlighted.

## An unrelated regression, found and fixed in passing

Inspecting the static frontend before wiring this in surfaced a real,
already-live bug: `cmd/server/static/index.html` still read
`r.score.toFixed(3)`, but `searchResult.Score` was removed in favor of
`Rank` back when hybrid ranking was first wired in (design doc 29). The
field simply didn't exist anymore - every real search threw inside the
render callback and silently showed nothing. Fixed alongside this
change (`rank` display instead of `score`), and verified by actually
running the server and driving the real page in a browser - not just
trusting the Go-side unit tests, which had no way to catch a
JavaScript-side field-name mismatch at all.

## Real proof

`TestSearchHandler_HighlightsMatchedTermInSnippetHTML` and
`TestRenderSnippetHTML_EscapesPlainTextAroundMatches` cover the
mechanism. Beyond that, the real combined-corpus server was run and
driven in an actual browser: a query for "sunscreen" rendered
highlighted matches across both corpora with ellipsis truncation
visible exactly where expected, and a query for "truffle" rendered the
real blogger's decorative Unicode-styled text with the match correctly
highlighted *and* the original styling preserved - a single screenshot
confirming both the Unicode-normalization fix (design doc 34) and the
offset-preservation guarantee (design doc 32) at once, in the real,
running UI rather than only in isolated tests.
