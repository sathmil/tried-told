package tokenize

import (
	"slices"
	"testing"
)

func TestTokenize(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{"letter to digit, no space", "SPF50", []string{"spf", "50"}},
		{"letter to digit, with space", "SPF 50", []string{"spf", "50"}},
		{"digit to letter, no space", "50ml", []string{"50", "ml"}},
		{"hyphen splits into two words", "non-comedogenic", []string{"non", "comedogenic"}},
		{"percent sign dropped", "10% niacinamide", []string{"10", "niacinamide"}},
		{"apostrophe splits contraction", "doesn't", []string{"doesn", "t"}},
		{"consecutive separators", "hello,,,world", []string{"hello", "world"}},
		{"empty string", "", nil},
		{"whitespace only", "     ", nil},
		{"punctuation only", "&*(@&#(", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Tokenize(tc.input)
			if !slices.Equal(got, tc.want) {
				t.Errorf("Tokenize(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestTokenizeWithOffsets_OffsetsSliceBackToTheOriginalToken(t *testing.T) {
	text := "SPF50 leaves no white cast"
	tokens := TokenizeWithOffsets(text)

	for _, tok := range tokens {
		gotSlice := text[tok.Start:tok.End]
		if !slices.Contains(Tokenize(gotSlice), tok.Text) {
			t.Errorf("text[%d:%d] = %q, does not tokenize to %q", tok.Start, tok.End, gotSlice, tok.Text)
		}
	}

	wantTexts := []string{"spf", "50", "leaves", "no", "white", "cast"}
	if len(tokens) != len(wantTexts) {
		t.Fatalf("got %d tokens, want %d", len(tokens), len(wantTexts))
	}
	for i, want := range wantTexts {
		if tokens[i].Text != want {
			t.Errorf("tokens[%d].Text = %q, want %q", i, tokens[i].Text, want)
		}
	}
}

// TestTokenizeWithOffsets_MultiByteRuneOffsetsAreByteNotRuneIndexed proves
// offsets are byte offsets (usable directly as text[start:end]), not rune
// counts - a real distinction once text contains any multi-byte UTF-8
// rune, and exactly the kind of assumption this project has been burned
// by trusting without checking before (e.g. the URL normalization %2F
// case).
func TestTokenizeWithOffsets_MultiByteRuneOffsetsAreByteNotRuneIndexed(t *testing.T) {
	text := "café latte" // "é" is 2 bytes in UTF-8, 1 rune
	tokens := TokenizeWithOffsets(text)

	if len(tokens) != 2 {
		t.Fatalf("got %d tokens, want 2", len(tokens))
	}
	if tokens[0].Text != "café" {
		t.Fatalf("tokens[0].Text = %q, want %q", tokens[0].Text, "café")
	}
	// "café" is 4 runes but 5 bytes (c-a-f-é), so if offsets were rune
	// counts instead of byte counts, this slice would cut "é" in half.
	if got, want := text[tokens[0].Start:tokens[0].End], "café"; got != want {
		t.Errorf("text[%d:%d] = %q, want %q", tokens[0].Start, tokens[0].End, got, want)
	}
	// "café" occupies bytes [0,5) (c,a,f = 1 byte each, é = 2 bytes), the
	// space is byte 5, so "latte" starts at byte 6 - if offsets were rune
	// counts, this would come out to 5 instead (4 runes + 1 space rune).
	if tokens[1].Start != 6 {
		t.Errorf("tokens[1].Start = %d, want 6 (byte offset, not rune offset)", tokens[1].Start)
	}
}
