package segment

import (
	"reflect"
	"testing"
)

func TestParagraphs(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "single paragraph is a no-op",
			text: "This sunscreen didn't leave a white cast on my deep skin.",
			want: []string{"This sunscreen didn't leave a white cast on my deep skin."},
		},
		{
			name: "two paragraphs split on the blank line",
			text: "First I tried this on my face.\n\nThen I tried it on my hands.",
			want: []string{"First I tried this on my face.", "Then I tried it on my hands."},
		},
		{
			name: "extra blank lines and surrounding whitespace are tolerated",
			text: "  First paragraph.  \n\n\n  Second paragraph.  ",
			want: []string{"First paragraph.", "Second paragraph."},
		},
		{
			name: "blank/whitespace-only paragraphs are dropped",
			text: "First paragraph.\n\n   \n\nSecond paragraph.",
			want: []string{"First paragraph.", "Second paragraph."},
		},
		{
			name: "empty input produces no paragraphs",
			text: "",
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Paragraphs(tc.text)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Paragraphs(%q) = %#v, want %#v", tc.text, got, tc.want)
			}
		})
	}
}
