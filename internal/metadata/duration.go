// Package metadata pulls explicitly stated structured context out of free
// review text, using rule-based pattern matching only - never inference or
// guessing. Per docs/design/16-structured-metadata.md.
package metadata

import "regexp"

// durationPattern matches a digit count immediately followed by a time
// unit (e.g. "3 weeks", "1 month"). Deliberately narrow: it does not match
// spelled-out numbers ("three weeks") or vague relative references
// ("a while", "for some time") - those aren't explicit enough to preserve
// without risking a wrong guess, so they're left unset rather than
// approximated.
var durationPattern = regexp.MustCompile(`(?i)\b\d+\s+(day|days|week|weeks|month|months|year|years)\b`)

// ExtractDuration returns the first explicitly stated duration-of-use
// phrase found in text verbatim (not normalized into a numeric+unit
// structure - the literal matched text is preserved as-is), and false if
// none is found.
func ExtractDuration(text string) (string, bool) {
	match := durationPattern.FindString(text)
	if match == "" {
		return "", false
	}
	return match, true
}
