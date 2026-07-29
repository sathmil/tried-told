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

	// Fourth review in the fixture has blank text and must be skipped.
	if len(passages) != 3 {
		t.Fatalf("got %d passages, want 3", len(passages))
	}

	want := []Passage{
		{
			Text:      "This sunscreen didn't leave a white cast on my deep skin, even after sweating in humid weather.",
			SourceURL: "https://example-reviews.test/product/1",
			Product:   "Hydrating Sunscreen SPF 50",
		},
		{
			Text:      "Left a chalky residue on my olive undertone, disappointing.",
			SourceURL: "https://example-reviews.test/product/1",
			Product:   "Hydrating Sunscreen SPF 50",
		},
		{
			Text:      "Brightened my skin within two weeks of daily use.",
			SourceURL: "https://example-reviews.test/product/1",
			Product:   "Vitamin C Serum",
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
