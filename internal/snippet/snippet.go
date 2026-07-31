// Package snippet extracts a highlighted excerpt of a passage's text
// around its best cluster of query-term matches, per
// docs/design/32-snippets.md.
package snippet

import "triedandtold/internal/tokenize"

// defaultWindowTokens is how many tokens (not characters or words) the
// extracted excerpt spans - a token-count budget, not a fixed character
// count, since it's directly comparable to the token positions Extract
// works with internally.
const defaultWindowTokens = 12

// Span is a match's byte offsets within Snippet.Text - relative to the
// extracted excerpt, not the original passage, since that's the only
// text a caller actually has to slice or mark up.
type Span struct {
	Start, End int
}

// Snippet is a highlighted excerpt of a passage's text.
type Snippet struct {
	Text    string
	Matches []Span
}

// Extract finds the window of text within text that best matches query
// and returns it as a Snippet, with Matches locating each query-term
// occurrence inside that window.
//
// Both text and query are tokenized with the exact same
// tokenize.TokenizeWithOffsets used for indexing and retrieval, rather
// than a separate substring or whitespace-based heuristic - so a term
// this considers "matched" for highlighting is always the same term
// that mattered for ranking. A naive substring search for "SPF 50"
// would miss a passage containing "SPF50" even though retrieval
// tokenizes both into the same ["spf","50"], which would silently fail
// to highlight a real match - the whole reason this package shares the
// tokenizer instead of re-implementing matching on its own.
func Extract(text, query string) Snippet {
	queryTerms := make(map[string]bool)
	for _, t := range tokenize.Tokenize(query) {
		queryTerms[t] = true
	}

	tokens := tokenize.TokenizeWithOffsets(text)
	if len(tokens) == 0 {
		return Snippet{Text: text}
	}

	windowSize := defaultWindowTokens
	if windowSize > len(tokens) {
		windowSize = len(tokens)
	}

	lo, _ := bestWindow(tokens, queryTerms, windowSize)
	hi := lo + windowSize

	windowStart := tokens[lo].Start
	windowEnd := tokens[hi-1].End
	snippetText := text[windowStart:windowEnd]

	var matches []Span
	for _, tok := range tokens[lo:hi] {
		if queryTerms[tok.Text] {
			matches = append(matches, Span{Start: tok.Start - windowStart, End: tok.End - windowStart})
		}
	}

	prefix, suffix := "", ""
	if lo > 0 {
		prefix = "… " // ellipsis - the window doesn't start at the passage's beginning
		for i := range matches {
			matches[i].Start += len(prefix)
			matches[i].End += len(prefix)
		}
	}
	if hi < len(tokens) {
		suffix = " …"
	}

	return Snippet{Text: prefix + snippetText + suffix, Matches: matches}
}

// bestWindow slides a fixed-size window of size tokens across the
// passage and returns the start index of the window containing the most
// query-term matches - not just the first window with any match at
// all, since a passage's strongest evidence for a query isn't always at
// its first occurrence. Ties favor the earliest window, for a
// deterministic result.
func bestWindow(tokens []tokenize.Token, queryTerms map[string]bool, size int) (start, score int) {
	matched := make([]bool, len(tokens))
	for i, tok := range tokens {
		matched[i] = queryTerms[tok.Text]
	}

	bestStart, bestScore := 0, -1
	current := 0
	for i := 0; i < size; i++ {
		if matched[i] {
			current++
		}
	}
	bestScore = current

	for i := size; i < len(tokens); i++ {
		if matched[i] {
			current++
		}
		if matched[i-size] {
			current--
		}
		windowStart := i - size + 1
		if current > bestScore {
			bestScore = current
			bestStart = windowStart
		}
	}

	return bestStart, bestScore
}
