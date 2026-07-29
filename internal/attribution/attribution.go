// Package attribution declares each source's provenance policy once,
// centrally, and connects it to what actually gets served - so a design
// doc's due-diligence notes about license/attribution/deletion terms are
// an enforced runtime fact, not just prose a human read once. Per
// docs/design/17-source-attribution.md.
package attribution

import (
	"fmt"
	"net/url"
)

// SourceType is a fixed, small set of provenance categories - never free
// text, so "is this opt-in or scraped" is always an exact, queryable fact,
// never a typo-prone string. Preventing exactly this mix-up is the whole
// point of this package: the project explicitly prohibits presenting a
// third-party review as an opt-in story.
type SourceType string

const (
	SourceTypeSynthetic       SourceType = "synthetic"
	SourceTypeOptIn           SourceType = "opt_in"
	SourceTypeLicensedDataset SourceType = "licensed_dataset"
	SourceTypePermittedCrawl  SourceType = "permitted_crawl"
)

// SourceInfo is one source's declared policy - the single place its
// license, attribution, and deletion terms live, rather than duplicated
// onto every passage that came from it.
type SourceInfo struct {
	Host                string
	Name                string
	Type                SourceType
	License             string
	AttributionRequired bool
	AttributionText     string
	DeletionContact     string
}

// Attribution returns the exact text that must accompany any display of
// content from this source, or "" if none is required.
func (s SourceInfo) Attribution() string {
	if !s.AttributionRequired {
		return ""
	}
	return s.AttributionText
}

// Registry maps a source's host to its declared policy.
type Registry struct {
	sources map[string]SourceInfo
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{sources: make(map[string]SourceInfo)}
}

// Register declares info for its host, overwriting any prior entry.
func (r *Registry) Register(info SourceInfo) {
	r.sources[info.Host] = info
}

// Lookup returns the declared SourceInfo for host, if any.
func (r *Registry) Lookup(host string) (SourceInfo, bool) {
	info, ok := r.sources[host]
	return info, ok
}

// MustLookup returns the declared SourceInfo for host, panicking if none
// exists. A deliberate guardrail: content from a host with no explicitly
// declared policy must never be served, so a missing registration fails
// loudly here rather than silently serving unlabeled or mislabeled content.
func (r *Registry) MustLookup(host string) SourceInfo {
	info, ok := r.Lookup(host)
	if !ok {
		panic(fmt.Sprintf("attribution: no source registered for host %q - every source must be explicitly registered before its content can be served", host))
	}
	return info
}

// HostOf extracts the "scheme://host" form used as the registry key
// throughout this project (matching internal/robots, internal/crawlstate)
// from a full URL.
func HostOf(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return u.Scheme + "://" + u.Host, nil
}
