package extract

import "testing"

func TestRegistry_RegisterAndLookup(t *testing.T) {
	r := NewRegistry()
	r.Register("https://example-reviews.test", ExampleSiteExtractor{})

	e, ok := r.ExtractorFor("https://example-reviews.test")
	if !ok {
		t.Fatal("expected an extractor registered for this host")
	}
	if _, ok := e.(ExampleSiteExtractor); !ok {
		t.Errorf("got extractor of type %T, want ExampleSiteExtractor", e)
	}
}

func TestRegistry_UnregisteredHostReturnsNotOK(t *testing.T) {
	r := NewRegistry()
	if _, ok := r.ExtractorFor("https://never-registered.test"); ok {
		t.Error("expected ok=false for an unregistered host")
	}
}
