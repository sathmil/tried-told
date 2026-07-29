package attribution

import "testing"

func exampleSource() SourceInfo {
	return SourceInfo{
		Host:                "https://example-reviews.test",
		Name:                "Example Reviews",
		Type:                SourceTypePermittedCrawl,
		License:             "Site ToS explicitly permits crawling and indexing (see docs/design/17)",
		AttributionRequired: true,
		AttributionText:     "Source: Example Reviews (example-reviews.test)",
		DeletionContact:     "privacy@example-reviews.test",
	}
}

func TestRegistry_RegisterAndLookup(t *testing.T) {
	r := NewRegistry()
	r.Register(exampleSource())

	info, ok := r.Lookup("https://example-reviews.test")
	if !ok {
		t.Fatal("expected a registered source")
	}
	if info.Name != "Example Reviews" || info.Type != SourceTypePermittedCrawl {
		t.Errorf("got %+v, want the registered SourceInfo", info)
	}
}

func TestRegistry_LookupUnregisteredHostReturnsNotOK(t *testing.T) {
	r := NewRegistry()
	if _, ok := r.Lookup("https://never-registered.test"); ok {
		t.Error("expected ok=false for an unregistered host")
	}
}

func TestRegistry_MustLookupPanicsForUnregisteredHost(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustLookup to panic for an unregistered host")
		}
	}()
	NewRegistry().MustLookup("https://never-registered.test")
}

func TestRegistry_MustLookupSucceedsForRegisteredHost(t *testing.T) {
	r := NewRegistry()
	r.Register(exampleSource())

	info := r.MustLookup("https://example-reviews.test") // must not panic
	if info.Name != "Example Reviews" {
		t.Errorf("got %+v, want the registered SourceInfo", info)
	}
}

func TestSourceInfo_AttributionRespectsRequiredFlag(t *testing.T) {
	required := exampleSource()
	if got := required.Attribution(); got != "Source: Example Reviews (example-reviews.test)" {
		t.Errorf("Attribution() = %q, want the declared attribution text", got)
	}

	notRequired := exampleSource()
	notRequired.AttributionRequired = false
	notRequired.AttributionText = "should never be shown"
	if got := notRequired.Attribution(); got != "" {
		t.Errorf("Attribution() = %q, want empty when not required", got)
	}
}

func TestHostOf(t *testing.T) {
	cases := []struct {
		rawURL string
		want   string
	}{
		{"https://example-reviews.test/product/1?a=1", "https://example-reviews.test"},
		{"http://example.com:8080/foo", "http://example.com:8080"},
	}
	for _, tc := range cases {
		got, err := HostOf(tc.rawURL)
		if err != nil {
			t.Fatalf("HostOf(%q) returned error: %v", tc.rawURL, err)
		}
		if got != tc.want {
			t.Errorf("HostOf(%q) = %q, want %q", tc.rawURL, got, tc.want)
		}
	}
}
