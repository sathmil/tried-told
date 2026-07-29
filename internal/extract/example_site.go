package extract

import (
	"strings"

	"github.com/PuerkitoBio/goquery"

	"triedandtold/internal/segment"
)

// ExampleSiteExtractor is a site-specific extractor for a hypothetical
// review site (see testdata/example_site.html), demonstrating the pattern:
// each review lives in a ".review" element with a "data-product"
// attribute, and its text in a nested ".review-text" element containing
// one or more paragraphs. Everything else on the page (nav, ads, footer)
// is simply never selected, rather than attempting to generically detect
// "boilerplate."
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

		for _, paragraph := range segment.Paragraphs(text) {
			passages = append(passages, Passage{
				Text:      paragraph,
				SourceURL: sourceURL,
				Product:   product,
			})
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
