package extract

import (
	"strings"

	"github.com/PuerkitoBio/goquery"

	"triedandtold/internal/metadata"
	"triedandtold/internal/segment"
)

// ExampleSiteExtractor is a site-specific extractor for a hypothetical
// review site (see testdata/example_site.html), demonstrating the pattern:
// each review lives in a ".review" element with "data-product" and
// "data-category" attributes (source-structured metadata), and its text in
// a nested ".review-text" element containing one or more paragraphs.
// Free-text metadata (currently duration of use) is pulled from the text
// itself via internal/metadata's rule-based extraction. Everything else on
// the page (nav, ads, footer) is simply never selected, rather than
// attempting to generically detect "boilerplate."
//
// Duration extraction is invoked directly here since this is currently the
// only extractor; once a second site-specific extractor exists, this
// should move to a shared post-extraction enrichment step so every future
// extractor gets it automatically rather than needing to remember to wire
// it in - see docs/design/16-structured-metadata.md.
type ExampleSiteExtractor struct{}

func (ExampleSiteExtractor) Extract(html, sourceURL string) ([]Passage, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	var passages []Passage
	doc.Find(".review").Each(func(_ int, s *goquery.Selection) {
		text := reviewText(s.Find(".review-text"))
		if text == "" {
			return // nothing to index - don't invent a passage from empty content
		}
		product, _ := s.Attr("data-product")
		category, _ := s.Attr("data-category")

		for _, paragraph := range segment.Paragraphs(text) {
			p := Passage{
				Text:            paragraph,
				SourceURL:       sourceURL,
				Product:         product,
				ProductCategory: category,
			}
			if duration, ok := metadata.ExtractDuration(paragraph); ok {
				p.DurationOfUse = duration
			}
			passages = append(passages, p)
		}
	})

	return passages, nil
}

// reviewText joins each <p> child of s with a blank line between them, so
// paragraph boundaries survive into a plain string instead of being lost.
// goquery's .Text() alone concatenates all descendant text nodes with no
// separator at all - verified directly, not assumed - which would merge
// words together across a paragraph break (e.g. "...was high" + "The
// quality..." -> "highThe"). Falls back to the selection's own .Text() if
// it has no nested <p> elements at all.
func reviewText(s *goquery.Selection) string {
	paragraphs := s.Find("p")
	if paragraphs.Length() == 0 {
		return strings.TrimSpace(s.Text())
	}

	var parts []string
	paragraphs.Each(func(_ int, p *goquery.Selection) {
		if t := strings.TrimSpace(p.Text()); t != "" {
			parts = append(parts, t)
		}
	})
	return strings.Join(parts, "\n\n")
}
