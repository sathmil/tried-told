package metadata

import "testing"

func TestExtractDuration(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		wantOK   bool
		wantText string
	}{
		{"weeks", "I've been using this for 3 weeks and it helped a lot.", true, "3 weeks"},
		{"singular month", "After 1 month my skin cleared up.", true, "1 month"},
		{"years", "I've used this for 2 years now.", true, "2 years"},
		{"case insensitive unit", "Used it for 5 DAYS straight.", true, "5 DAYS"},
		{"no duration mentioned", "This sunscreen didn't leave a white cast on my deep skin.", false, ""},
		{"spelled-out number is deliberately not matched", "I used this for three weeks.", false, ""},
		{"vague relative reference is deliberately not matched", "I've used this for a while now.", false, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ExtractDuration(tc.text)
			if ok != tc.wantOK {
				t.Fatalf("ExtractDuration(%q) ok = %v, want %v", tc.text, ok, tc.wantOK)
			}
			if got != tc.wantText {
				t.Errorf("ExtractDuration(%q) = %q, want %q", tc.text, got, tc.wantText)
			}
		})
	}
}
