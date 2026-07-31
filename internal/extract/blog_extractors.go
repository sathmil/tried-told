package extract

import (
	"strings"

	"github.com/PuerkitoBio/goquery"

	"triedandtold/internal/metadata"
	"triedandtold/internal/segment"
	"triedandtold/internal/tokenize"
)

// entryContentExtract is the shared logic behind WordPressBlogExtractor
// and BloggerBlogExtractor: a single blog post page, one post body per
// page (unlike ExampleSiteExtractor's multi-review page), with no
// structured product/category markup available - real blog HTML doesn't
// tag itself the way a review-aggregator site's data attributes do, so
// those Passage fields are left empty rather than guessed from prose.
func entryContentExtract(html, sourceURL string) ([]Passage, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	text := reviewText(doc.Find(".entry-content").First())
	if text == "" {
		return nil, nil
	}

	var passages []Passage
	for _, paragraph := range segment.Paragraphs(text) {
		// A real WordPress post surfaced this: some "paragraphs" are
		// purely decorative (a lone emoji used as a section divider) -
		// non-empty text, but zero actual letters/digits to index.
		// segment.Paragraphs only drops truly-empty strings, which isn't
		// the same check - tokenize.Tokenize is what actually determines
		// whether there's anything for BM25 or embeddings to work with,
		// so that's the right test here, not string emptiness.
		if len(tokenize.Tokenize(paragraph)) == 0 {
			continue
		}
		p := Passage{Text: paragraph, SourceURL: sourceURL}
		if duration, ok := metadata.ExtractDuration(paragraph); ok {
			p.DurationOfUse = duration
		}
		passages = append(passages, p)
	}
	return passages, nil
}

// WordPressBlogExtractor is a site-specific extractor for individual
// WordPress.com-hosted blog post pages. Verified directly against a real
// fetched page (testdata/simplyemsblog_dalba_sunscreen_review.html,
// simplyemsblog.wordpress.com, captured 2026-07-30): the post body lives
// in a single ".entry-content" element.
type WordPressBlogExtractor struct{}

func (WordPressBlogExtractor) Extract(html, sourceURL string) ([]Passage, error) {
	return entryContentExtract(html, sourceURL)
}

// BloggerBlogExtractor is a site-specific extractor for individual
// Blogger-hosted blog post pages. Verified directly against a real
// fetched page (testdata/stylexplora_skinfood_apothecary.html,
// stylexplora.blogspot.com, captured 2026-07-30): this theme's post body
// carries class="post-body entry-content float-container", so the same
// ".entry-content" selector applies - confirmed by inspecting the real
// page, not assumed just because the class name happens to match
// WordPress's convention too.
type BloggerBlogExtractor struct{}

func (BloggerBlogExtractor) Extract(html, sourceURL string) ([]Passage, error) {
	return entryContentExtract(html, sourceURL)
}
