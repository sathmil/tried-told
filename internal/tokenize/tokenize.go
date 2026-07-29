// Package tokenize splits and normalizes raw text into index terms,
// per docs/design/02-tokenization.md.
package tokenize

import (
	"strings"
	"unicode"
)

// Tokenize splits text into lowercased terms. A term is a maximal run of
// letters, or a maximal run of digits. Any other rune (whitespace,
// punctuation, hyphens, '%', etc.) is a boundary and is dropped. A
// transition between a letter-run and a digit-run is also a boundary,
// so "SPF50" and "SPF 30" tokenize consistently.
func Tokenize(text string) []string {
	var tokens []string
	var current []rune
	currentIsDigit := false // which class the run in `current` belongs to

	flush := func() {
		if len(current) > 0 {
			tokens = append(tokens, strings.ToLower(string(current)))
			current = current[:0]
		}
	}

	for _, r := range text {
		switch {
		case unicode.IsLetter(r):
			if len(current) > 0 && currentIsDigit {
				flush()
			}
			currentIsDigit = false
			current = append(current, r)
		case unicode.IsDigit(r):
			if len(current) > 0 && !currentIsDigit {
				flush()
			}
			currentIsDigit = true
			current = append(current, r)
		default:
			flush()
		}
	}
	flush()

	return tokens
}
