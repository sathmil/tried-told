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
