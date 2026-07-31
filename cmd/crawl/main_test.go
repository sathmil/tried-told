package main

import (
	"slices"
	"testing"
)

func TestDiscoverPostLinks_MatchesKnownPostShapeOnTheSameHost(t *testing.T) {
	html := `
		<a href="/2023/01/19/olens-double-tint-review/">a real post</a>
		<a href="/category/beauty/page/2/">pagination, not a post</a>
		<a href="/2023/01/19/olens-double-tint-review/">duplicate of the first link</a>
		<a href="https://other-site.example/2023/01/19/not-this-host/">wrong host entirely</a>
	`
	got := discoverPostLinks(html, "https://simplyemsblog.wordpress.com/category/beauty/")
	want := []string{"https://simplyemsblog.wordpress.com/2023/01/19/olens-double-tint-review/"}

	if !slices.Equal(got, want) {
		t.Errorf("discoverPostLinks(...) = %v, want %v", got, want)
	}
}

// TestDiscoverPostLinks_HandlesFullyAbsoluteHrefs covers the shape both
// real vetted sites actually use in practice (verified directly against
// their real HTML): every post link is a fully-qualified
// "https://host/path" href, not a root- or document-relative one. A
// document-relative href (no leading slash) resolves against the
// *base's directory*, not its domain root - real content here never
// exercises that path, so this test sticks to the case that matters.
func TestDiscoverPostLinks_HandlesFullyAbsoluteHrefs(t *testing.T) {
	html := `<a href="https://simplyemsblog.wordpress.com/2023/01/19/olens-double-tint-review/">absolute link</a>`
	got := discoverPostLinks(html, "https://simplyemsblog.wordpress.com/category/beauty/")
	want := []string{"https://simplyemsblog.wordpress.com/2023/01/19/olens-double-tint-review/"}

	if !slices.Equal(got, want) {
		t.Errorf("discoverPostLinks(...) = %v, want %v", got, want)
	}
}

func TestDiscoverPostLinks_UnregisteredHostReturnsNothing(t *testing.T) {
	html := `<a href="/2023/01/19/some-post/">a post-shaped link</a>`
	got := discoverPostLinks(html, "https://not-a-vetted-site.example/some/page/")

	if got != nil {
		t.Errorf("discoverPostLinks(...) = %v, want nil - this host isn't in postURLPattern at all", got)
	}
}
