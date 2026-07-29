package extract

import (
	"os"
	"strings"
	"testing"
)

func TestExampleSiteExtractor_ExtractsPassagesAndSkipsBoilerplate(t *testing.T) {
	html, err := os.ReadFile("testdata/example_site.html")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	passages, err := ExampleSiteExtractor{}.Extract(string(html), "https://example-reviews.test/product/1")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	// 3 single-paragraph reviews + the multi-paragraph review split into 2
	// passages; the blank-text review is skipped entirely.
	if len(passages) != 5 {
		t.Fatalf("got %d passages, want 5", len(passages))
	}

	want := []Passage{
		{
			Text:            "This sunscreen didn't leave a white cast on my deep skin, even after sweating in humid weather.",
			SourceURL:       "https://example-reviews.test/product/1",
			Product:         "Hydrating Sunscreen SPF 50",
			ProductCategory: "Sunscreen",
		},
		{
			Text:            "Left a chalky residue on my olive undertone, disappointing.",
			SourceURL:       "https://example-reviews.test/product/1",
			Product:         "Hydrating Sunscreen SPF 50",
			ProductCategory: "Sunscreen",
		},
		{
			Text:            "Brightened my skin within 2 weeks of daily use.",
			SourceURL:       "https://example-reviews.test/product/1",
			Product:         "Vitamin C Serum",
			ProductCategory: "Serum",
			DurationOfUse:   "2 weeks",
		},
		{
			Text:            "First I tried this on my face for two weeks and it felt lightweight.",
			SourceURL:       "https://example-reviews.test/product/1",
			Product:         "Multi Paragraph Moisturizer",
			ProductCategory: "Moisturizer",
			// "two weeks" is spelled out, not digit-based - deliberately not
			// matched by ExtractDuration, so DurationOfUse stays empty here.
		},
		{
			Text:            "Then I noticed it also helped soften the dry patches on my hands.",
			SourceURL:       "https://example-reviews.test/product/1",
			Product:         "Multi Paragraph Moisturizer",
			ProductCategory: "Moisturizer",
		},
	}
	for i, w := range want {
		if passages[i] != w {
			t.Errorf("passage %d = %+v, want %+v", i, passages[i], w)
		}
	}

	// Boilerplate (nav/ad/footer text) must never leak into any passage.
	boilerplateFragments := []string{"Home | Products", "Limited time offer", "All rights reserved"}
	for _, p := range passages {
		for _, frag := range boilerplateFragments {
			if strings.Contains(p.Text, frag) {
				t.Errorf("passage %+v unexpectedly contains boilerplate fragment %q", p, frag)
			}
		}
	}

	// The bug this whole segmentation pass caught: without an explicit
	// paragraph separator, the two paragraphs of the multi-paragraph review
	// would merge into one run with no space - "lightweight.Then" - once
	// goquery flattened them. Confirm that never happens in any passage.
	for _, p := range passages {
		if strings.Contains(p.Text, "lightweight.Then") {
			t.Errorf("passage %+v has merged paragraph text with no separator", p)
		}
	}
}

func TestExampleSiteExtractor_NoReviewsReturnsEmpty(t *testing.T) {
	passages, err := ExampleSiteExtractor{}.Extract("<html><body><nav>nothing here</nav></body></html>", "https://example-reviews.test/empty")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if len(passages) != 0 {
		t.Errorf("got %d passages, want 0", len(passages))
	}
}
