// Package language detects a passage's language, so mixed-language content
// never silently pollutes BM25 statistics and the corpus's actual language
// mix becomes a measurable fact rather than an assumption. Per
// docs/design/15-language-detection.md.
package language

import (
	"strings"
	"unicode/utf8"

	"github.com/pemistahl/lingua-go"
)

// MinChars is the minimum rune count Detect will trust any result for,
// regardless of what the underlying library reports. Verified directly
// that very short input (e.g. 3 characters) can still produce a
// confident-looking result that's statistically meaningless - the
// library's own confidence check guards against ambiguity between
// candidate languages, not against there simply not being enough text to
// carry any real signal.
const MinChars = 20

var detector = lingua.NewLanguageDetectorBuilder().FromAllLanguages().Build()

// Detect returns the ISO 639-1 code (e.g. "en") for text's language, and
// false if text is too short to trust or the language can't be reliably
// determined. Never invents a language for text we're not confident about -
// same "don't invent missing metadata" principle applied to every other
// field on Passage.
func Detect(text string) (string, bool) {
	if utf8.RuneCountInString(text) < MinChars {
		return "", false
	}
	lang, ok := detector.DetectLanguageOf(text)
	if !ok {
		return "", false
	}
	// The library's own String() returns uppercase ("EN"); lowercased here
	// to match the conventional ISO 639-1 form ("en") used elsewhere (e.g.
	// HTTP Accept-Language).
	return strings.ToLower(lang.IsoCode639_1().String()), true
}
