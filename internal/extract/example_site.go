package extract

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ExampleSiteExtractor is a site-specific extractor for a hypothetical
// review site (see testdata/example_site.html), demonstrating the pattern:
// each review lives in a ".review" element with a "data-product"
// attribute, and the review text itself in a nested ".review-text"
// element. Everything else on the page (nav, ads, footer) is simply never
// selected, rather than attempting to generically detect "boilerplate."
type ExampleSiteExtractor struct{}

func (ExampleSiteExtractor) Extract(html, sourceURL string) ([]Passage, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	var passages []Passage
	doc.Find(".review").Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Find(".review-text").Text())
		if text == "" {
			return // nothing to index - don't invent a passage from empty content
		}
		product, _ := s.Attr("data-product")
		passages = append(passages, Passage{
			Text:      text,
			SourceURL: sourceURL,
			Product:   product,
		})
	})

	return passages, nil
}
