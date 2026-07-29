// Package extract turns one fetched HTML page (a container - a product
// page, a forum thread) into individually-searchable passages, per
// docs/design/12-extraction.md.
package extract

import (
	"crypto/sha256"
	"encoding/hex"
)

// Passage is one extracted, individually-searchable unit - a single review
// or story, not a whole page. Metadata fields are left empty rather than
// guessed when a page doesn't explicitly state them - never invent what
// isn't there.
type Passage struct {
	Text            string
	SourceURL       string
	Product         string
	ProductCategory string
	SkinTone        string
	Climate         string
	DurationOfUse   string // explicitly stated in the text, e.g. "3 weeks" - see internal/metadata
}

// ID deterministically identifies this passage by its content: the same
// SourceURL + Text always produces the same ID, so re-extracting unchanged
// content is idempotent, while a genuine text edit produces a different ID
// entirely - content-based identity was chosen over position-based so an
// edited review is treated as a new state to re-check, not silently
// assumed to still be "the same" passage. See
// docs/design/18-deletion-reindexing.md.
func (p Passage) ID() string {
	// "\x00" separates the two fields so concatenation can't collide -
	// without it, SourceURL="http://a.com/b"+Text="c" and
	// SourceURL="http://a.com"+Text="/bc" would hash identically.
	sum := sha256.Sum256([]byte(p.SourceURL + "\x00" + p.Text))
	return hex.EncodeToString(sum[:])
}

// Extractor turns one fetched page's HTML into its constituent passages.
// Implementations are site-specific: each knows the DOM structure of the
// one source it was written for, rather than attempting to generically
// detect "boilerplate" on arbitrary, unknown pages.
type Extractor interface {
	Extract(html, sourceURL string) ([]Passage, error)
}

// Registry maps a source's host to the Extractor that knows its structure.
type Registry struct {
	extractors map[string]Extractor
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{extractors: make(map[string]Extractor)}
}

// Register associates host with e. host should match the form used
// elsewhere in this project (scheme://host), e.g. "https://example.com".
func (r *Registry) Register(host string, e Extractor) {
	r.extractors[host] = e
}

// ExtractorFor returns the Extractor registered for host, if any.
func (r *Registry) ExtractorFor(host string) (Extractor, bool) {
	e, ok := r.extractors[host]
	return e, ok
}
