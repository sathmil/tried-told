// Package segment splits extracted text into passage-sized units at
// paragraph boundaries, per docs/design/14-passage-segmentation.md.
package segment

import (
	"regexp"
	"strings"
)

// blankLine matches one or more blank lines (possibly with trailing
// whitespace) separating paragraphs.
var blankLine = regexp.MustCompile(`\n\s*\n`)

// Paragraphs splits text at paragraph boundaries (blank lines), trimming
// whitespace and dropping empty results. Text with no blank-line breaks
// returns as a single paragraph unchanged - a no-op for content that's
// already just one paragraph, which is most of what this corpus has.
func Paragraphs(text string) []string {
	var out []string
	for _, part := range blankLine.Split(text, -1) {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
