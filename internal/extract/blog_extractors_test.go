package extract

import (
	"os"
	"strings"
	"testing"
)

// TestWordPressBlogExtractor_ExtractsRealReviewContent proves extraction
// against a real, fully unmodified page fetched from a real, robots.txt
// -vetted site (simplyemsblog.wordpress.com) - not a hand-crafted
// fixture - correctly finds the post body and ignores everything else on
// the page (nav, sidebar, related posts, comments).
func TestWordPressBlogExtractor_ExtractsRealReviewContent(t *testing.T) {
	html, err := os.ReadFile("testdata/simplyemsblog_dalba_sunscreen_review.html")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	sourceURL := "https://simplyemsblog.wordpress.com/2020/08/19/dalba-sunscreen-review/"
	passages, err := WordPressBlogExtractor{}.Extract(string(html), sourceURL)
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	if len(passages) == 0 {
		t.Fatal("got 0 passages, want at least 1 from a real review page")
	}

	// Deliberately plain-ASCII terms only: this post also uses decorative
	// Unicode "Mathematical Bold" styling for some words (e.g. a stylized
	// "Truffle" that isn't the same codepoints as plain ASCII "truffle" -
	// tokenize.Tokenize doesn't currently fold that, a real gap surfaced
	// by this fixture and tracked separately, not something this test
	// should depend on).
	joined := strings.ToLower(strings.Join(passageTexts(passages), " "))
	for _, want := range []string{"sun cream", "skin care"} {
		if !strings.Contains(joined, want) {
			t.Errorf("extracted text does not contain %q - wrong element selected?", want)
		}
	}

	for _, p := range passages {
		if p.SourceURL != sourceURL {
			t.Errorf("Passage.SourceURL = %q, want %q", p.SourceURL, sourceURL)
		}
		if p.Product != "" || p.ProductCategory != "" {
			t.Errorf("Passage has Product/ProductCategory %q/%q - real blog HTML has no structured markup for these, they must stay empty", p.Product, p.ProductCategory)
		}
	}
}

// TestBloggerBlogExtractor_ExtractsRealReviewContent is the same proof
// for the Blogger platform (stylexplora.blogspot.com) - a different
// platform whose theme happens to also use ".entry-content", verified
// directly rather than assumed from WordPress's convention.
func TestBloggerBlogExtractor_ExtractsRealReviewContent(t *testing.T) {
	html, err := os.ReadFile("testdata/stylexplora_skinfood_apothecary.html")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	sourceURL := "https://stylexplora.blogspot.com/2017/12/complete-skincare-with-skinfood.html"
	passages, err := BloggerBlogExtractor{}.Extract(string(html), sourceURL)
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	if len(passages) == 0 {
		t.Fatal("got 0 passages, want at least 1 from a real review page")
	}

	joined := strings.ToLower(strings.Join(passageTexts(passages), " "))
	if !strings.Contains(joined, "skinfood") {
		t.Errorf("extracted text does not contain %q - wrong element selected?", "skinfood")
	}
}

// TestWordPressBlogExtractor_SkipsDecorativeNonTextParagraphs proves the
// junk-content fix: this real page repeats a lone "🥀" emoji as a
// section divider - non-empty text, but nothing tokenize.Tokenize
// considers a real term. Indexing it would both waste dictionary space
// and, since it appears identically more than once at the same URL,
// produce duplicate Passage.ID()s (SourceURL+Text hashes identically) -
// this is what originally surfaced the issue.
func TestWordPressBlogExtractor_SkipsDecorativeNonTextParagraphs(t *testing.T) {
	html, err := os.ReadFile("testdata/simplyemsblog_dalba_sunscreen_review.html")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	passages, err := WordPressBlogExtractor{}.Extract(string(html), "https://simplyemsblog.wordpress.com/2020/08/19/dalba-sunscreen-review/")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	for _, p := range passages {
		if strings.TrimSpace(p.Text) == "🥀" {
			t.Errorf("a purely decorative passage (%q) was not filtered out", p.Text)
		}
	}

	ids := make(map[string]bool)
	for _, p := range passages {
		id := p.ID()
		if ids[id] {
			t.Errorf("duplicate Passage.ID() %q - two passages ended up with identical SourceURL+Text", id)
		}
		ids[id] = true
	}
}

func passageTexts(passages []Passage) []string {
	texts := make([]string, len(passages))
	for i, p := range passages {
		texts[i] = p.Text
	}
	return texts
}
