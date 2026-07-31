package snippet

import (
	"strings"
	"testing"
)

// TestExtract_MatchesShareTheRetrievalTokenizerNotLiteralSubstring is the
// proof that actually justifies this design: the query "SPF 50" (two
// words, space-separated) must still find a match inside "SPF50" (no
// space) in the passage, because both tokenize into the identical
// ["spf","50"] - the same reason retrieval matches them. A literal
// substring search for "SPF 50" would never find this at all.
func TestExtract_MatchesShareTheRetrievalTokenizerNotLiteralSubstring(t *testing.T) {
	s := Extract("This SPF50 sunscreen absorbed fast.", "SPF 50")

	if !strings.Contains(strings.ToLower(s.Text), "spf50") {
		t.Fatalf("Snippet.Text = %q, want it to contain the matched passage text", s.Text)
	}
	if len(s.Matches) != 2 {
		t.Fatalf("got %d matches, want 2 (spf and 50 both matched)", len(s.Matches))
	}
	for _, m := range s.Matches {
		got := strings.ToLower(s.Text[m.Start:m.End])
		if got != "spf" && got != "50" {
			t.Errorf("match span %v = %q, want \"spf\" or \"50\"", m, got)
		}
	}
}

// TestExtract_PicksHighestDensityWindowNotFirstMatch proves the window
// selection is based on match density, not just "wherever the first hit
// happens to be": an isolated early match must lose to a later, denser
// cluster of matches.
func TestExtract_PicksHighestDensityWindowNotFirstMatch(t *testing.T) {
	// Token layout: one isolated "sunscreen" at index 2 (early), then a
	// cluster of three at indices 12, 14, 16 (late) - see
	// docs/design/32-snippets.md for the worked-out window math.
	text := "one match sunscreen two three four five six seven eight nine ten sunscreen cast sunscreen cast sunscreen eleven twelve"

	s := Extract(text, "sunscreen")

	if len(s.Matches) != 3 {
		t.Fatalf("got %d matches, want 3 (the dense later cluster, not the single early match)", len(s.Matches))
	}
	if strings.Contains(s.Text, "one match") {
		t.Errorf("Snippet.Text = %q, should not include the early isolated match's low-density window", s.Text)
	}
}

// TestExtract_NoMatchFallsBackToStartOfText covers a real case: a
// passage surfaced purely by semantic search may share no vocabulary
// with the query at all. Extract must still return something useful
// (the start of the text), not an empty or arbitrary result.
func TestExtract_NoMatchFallsBackToStartOfText(t *testing.T) {
	text := "a b c d e f g h i j k l m n o"
	s := Extract(text, "sunscreen")

	if len(s.Matches) != 0 {
		t.Errorf("got %d matches, want 0 (query term never appears)", len(s.Matches))
	}
	if !strings.HasPrefix(s.Text, "a b c") {
		t.Errorf("Snippet.Text = %q, want it to start from the beginning of the passage", s.Text)
	}
}

// TestExtract_ShortPassageReturnsWholeTextUnchanged confirms a passage
// shorter than the window isn't truncated with ellipses it doesn't need.
func TestExtract_ShortPassageReturnsWholeTextUnchanged(t *testing.T) {
	text := "short passage with few words"
	s := Extract(text, "passage")

	if s.Text != text {
		t.Errorf("Snippet.Text = %q, want the original text unchanged: %q", s.Text, text)
	}
	if len(s.Matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(s.Matches))
	}
	if got := s.Text[s.Matches[0].Start:s.Matches[0].End]; got != "passage" {
		t.Errorf("match span = %q, want \"passage\"", got)
	}
}
