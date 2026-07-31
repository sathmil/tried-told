// Package tokenize splits and normalizes raw text into index terms,
// per docs/design/02-tokenization.md.
package tokenize

import (
	"strings"
	"unicode"
)

// Token is one term plus the byte offsets it occupied in the original
// text it was extracted from - Start/End index into that original
// string directly (e.g. text[t.Start:t.End]), not into Text, and not a
// rune count (a token containing multi-byte runes needs byte offsets to
// slice the original UTF-8 string correctly).
type Token struct {
	Text  string
	Start int
	End   int
}

// Tokenize splits text into lowercased terms. A term is a maximal run of
// letters, or a maximal run of digits. Any other rune (whitespace,
// punctuation, hyphens, '%', etc.) is a boundary and is dropped. A
// transition between a letter-run and a digit-run is also a boundary,
// so "SPF50" and "SPF 30" tokenize consistently.
//
// Defined in terms of TokenizeWithOffsets rather than as a separate
// implementation, so the two can never quietly disagree about what
// counts as a term - a real risk if snippet highlighting (which needs
// offsets) re-implemented tokenization on its own instead of sharing
// this logic (see docs/design/32-snippets.md).
func Tokenize(text string) []string {
	withOffsets := TokenizeWithOffsets(text)
	if len(withOffsets) == 0 {
		return nil
	}
	tokens := make([]string, len(withOffsets))
	for i, t := range withOffsets {
		tokens[i] = t.Text
	}
	return tokens
}

// TokenizeWithOffsets applies the same rules as Tokenize, but also
// records each token's byte span in the original text - the information
// snippet highlighting needs to slice out and mark up exactly the text
// that matched, using the identical notion of "match" the index itself
// uses.
func TokenizeWithOffsets(text string) []Token {
	var tokens []Token
	var current []rune
	currentIsDigit := false // which class the run in `current` belongs to
	start := 0              // byte offset where the run in `current` began

	flush := func(end int) {
		if len(current) > 0 {
			tokens = append(tokens, Token{
				Text:  strings.ToLower(string(current)),
				Start: start,
				End:   end,
			})
			current = current[:0]
		}
	}

	for i, r := range text {
		switch {
		case unicode.IsLetter(r):
			if len(current) > 0 && currentIsDigit {
				flush(i)
			}
			if len(current) == 0 {
				start = i
			}
			currentIsDigit = false
			current = append(current, r)
		case unicode.IsDigit(r):
			if len(current) > 0 && !currentIsDigit {
				flush(i)
			}
			if len(current) == 0 {
				start = i
			}
			currentIsDigit = true
			current = append(current, r)
		default:
			flush(i)
		}
	}
	flush(len(text))

	return tokens
}
