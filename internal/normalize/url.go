// Package normalize canonicalizes URLs so equivalent spellings collapse to
// one dedup key, per docs/design/07-url-normalization.md.
package normalize

import (
	"net/url"
	"path"
	"strconv"
	"strings"
)

// URL returns the canonical form of rawURL:
//   - scheme and host lowercased
//   - default port removed (:80 on http, :443 on https)
//   - fragment stripped (the server never sees it anyway)
//   - percent-encoded unreserved characters decoded, remaining escapes
//     uppercased (reserved characters, e.g. an encoded "/" as %2F, are
//     never decoded - doing so would change the path's actual structure)
//   - "."/".." path segments resolved
//   - query parameters sorted by key
//
// Left deliberately unmerged (see design doc): trailing-slash presence,
// and www vs non-www hosts - both are heuristic risks rather than safe
// rewrites, and are left to redirect-following / near-duplicate detection
// instead.
func URL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = normalizeHost(u.Hostname(), u.Port(), u.Scheme)
	u.Fragment = ""

	if escaped := u.EscapedPath(); escaped != "" {
		escaped = normalizePercentEncoding(escaped)
		escaped = cleanPath(escaped)
		decoded, err := url.PathUnescape(escaped)
		if err != nil {
			return "", err
		}
		u.RawPath = escaped
		u.Path = decoded
	}

	if u.RawQuery != "" {
		u.RawQuery = u.Query().Encode() // url.Values.Encode sorts by key
	}

	return u.String(), nil
}

func normalizeHost(host, port, scheme string) string {
	host = strings.ToLower(host)
	if port == "" || isDefaultPort(scheme, port) {
		return host
	}
	return host + ":" + port
}

func isDefaultPort(scheme, port string) bool {
	switch scheme {
	case "http":
		return port == "80"
	case "https":
		return port == "443"
	}
	return false
}

// cleanPath resolves "."/".." segments while preserving whether the
// original path ended in "/" - path.Clean alone would silently strip a
// trailing slash, which would undo the deliberate decision not to merge
// "/foo" and "/foo/". Operates on the escaped path string, so a still-
// escaped reserved character like %2F is never mistaken for a real "/"
// separator (it contains no literal "/" byte).
func cleanPath(p string) string {
	hadTrailingSlash := len(p) > 1 && strings.HasSuffix(p, "/")
	cleaned := path.Clean(p)
	if hadTrailingSlash && !strings.HasSuffix(cleaned, "/") {
		cleaned += "/"
	}
	return cleaned
}

// normalizePercentEncoding decodes percent-encoded unreserved characters
// (RFC 3986 unreserved set: ALPHA / DIGIT / "-" / "." / "_" / "~") and
// uppercases the hex digits of any escape it leaves in place. Reserved
// characters are never decoded, since doing so could change the path's
// structure (e.g. an encoded "/" is not the same as a real separator).
func normalizePercentEncoding(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2]) {
			hexDigits := s[i+1 : i+3]
			val, err := strconv.ParseUint(hexDigits, 16, 8)
			if err == nil && isUnreserved(byte(val)) {
				b.WriteByte(byte(val))
			} else {
				b.WriteByte('%')
				b.WriteString(strings.ToUpper(hexDigits))
			}
			i += 2
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isUnreserved(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') ||
		b == '-' || b == '.' || b == '_' || b == '~'
}
