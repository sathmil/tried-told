package normalize

import "testing"

func TestURL(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"lowercases scheme and host, preserves path case", "HTTP://Example.COM/Foo", "http://example.com/Foo"},
		{"removes default http port", "http://example.com:80/foo", "http://example.com/foo"},
		{"removes default https port", "https://example.com:443/foo", "https://example.com/foo"},
		{"keeps non-default port", "http://example.com:8080/foo", "http://example.com:8080/foo"},
		{"strips fragment", "http://example.com/foo#section", "http://example.com/foo"},
		{"resolves dot segments", "http://example.com/foo/../bar", "http://example.com/bar"},
		{"preserves present trailing slash", "http://example.com/foo/", "http://example.com/foo/"},
		{"preserves absent trailing slash after dot resolution", "http://example.com/foo/./bar", "http://example.com/foo/bar"},
		{"preserves trailing slash through dot resolution", "http://example.com/foo/../bar/", "http://example.com/bar/"},
		{"sorts query parameters", "http://example.com/foo?b=2&a=1", "http://example.com/foo?a=1&b=2"},
		{"does not merge www", "http://www.example.com/foo", "http://www.example.com/foo"},
		{"introduces no query string when there is none", "http://example.com/foo", "http://example.com/foo"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := URL(tc.input)
			if err != nil {
				t.Fatalf("URL(%q) returned error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("URL(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestURL_PercentEncoding documents actual observed behavior from Go's
// net/url round-trip, rather than assumed RFC 3986 behavior - see
// docs/design/07-url-normalization.md for why this is verified, not assumed.
func TestURL_PercentEncoding(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"unreserved percent-encoded char", "http://example.com/%7Euser", "http://example.com/~user"},
		{"lowercase hex escape", "http://example.com/foo%2fbar", "http://example.com/foo%2Fbar"},
		{
			// "a%2Fb" must stay one atomic segment (not split into "a"/"b"),
			// and ".." must cancel only the adjacent "c" segment, not reach
			// past it into "a%2Fb".
			"encoded slash is never treated as a real separator",
			"http://example.com/a%2Fb/c/../d",
			"http://example.com/a%2Fb/d",
		},
		{
			"encoded dot segments still get resolved (both unreserved, safe to decode first)",
			"http://example.com/foo/%2E%2E/bar",
			"http://example.com/bar",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := URL(tc.input)
			if err != nil {
				t.Fatalf("URL(%q) returned error: %v", tc.input, err)
			}
			t.Logf("URL(%q) = %q", tc.input, got)
			if got != tc.want {
				t.Errorf("URL(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
