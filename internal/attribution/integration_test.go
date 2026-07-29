package attribution

import (
	"os"
	"testing"

	"triedandtold/internal/extract"
)

// TestPassageFromExtractorResolvesToDeclaredSource proves the actual
// wiring end-to-end: a real Passage produced by ExampleSiteExtractor
// resolves, via its SourceURL, to the policy declared for that host - not
// just that the registry works in isolation.
func TestPassageFromExtractorResolvesToDeclaredSource(t *testing.T) {
	html, err := os.ReadFile("../extract/testdata/example_site.html")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	passages, err := extract.ExampleSiteExtractor{}.Extract(string(html), "https://example-reviews.test/product/1")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if len(passages) == 0 {
		t.Fatal("expected at least one extracted passage")
	}

	registry := NewRegistry()
	registry.Register(exampleSource())

	for _, p := range passages {
		host, err := HostOf(p.SourceURL)
		if err != nil {
			t.Fatalf("HostOf(%q) returned error: %v", p.SourceURL, err)
		}

		info := registry.MustLookup(host) // must not panic
		if info.Type != SourceTypePermittedCrawl {
			t.Errorf("passage %+v resolved to source type %v, want %v", p, info.Type, SourceTypePermittedCrawl)
		}
		if got := info.Attribution(); got == "" {
			t.Errorf("passage %+v resolved to a source with no attribution text", p)
		}
	}
}
