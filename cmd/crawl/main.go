// Command crawl runs a real, deliberately bounded crawl against a fixed
// list of individually vetted real pages - see
// docs/design/33-real-crawl.md for how each source was checked
// (robots.txt fetched and read directly, platform terms checked for any
// scraping restriction) before being added below. This is not an
// open/link-following crawl: it only ever fetches exactly the URLs
// listed in seeds, nothing discovered along the way.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/url"
	"os"
	"time"

	"triedandtold/internal/dedup"
	"triedandtold/internal/diskindex"
	"triedandtold/internal/extract"
	"triedandtold/internal/fetch"
	"triedandtold/internal/frontier"
	"triedandtold/internal/robots"
)

// seeds is the complete, fixed list of pages this crawl fetches. Vetted
// 2026-07-30: both hosts' robots.txt were fetched and read directly
// (User-agent: * allows normal crawling of post content on both; only
// admin/login/search paths are disallowed), and neither platform's terms
// of service prohibits automated access - WordPress.com's ToS only warns
// against overburdening its infrastructure, which this crawl's 5-second
// per-host delay and single worker comfortably respects.
var seeds = []string{
	"https://simplyemsblog.wordpress.com/2020/08/19/dalba-sunscreen-review/",
	"https://simplyemsblog.wordpress.com/2020/09/01/cosrx-vitamin-c-serum-review/",
	"https://simplyemsblog.wordpress.com/2020/08/17/isntree-hyaluronic-acid-mask-review/",
	"https://simplyemsblog.wordpress.com/2020/06/17/pyunkang-yul-toner-review/",
	"https://stylexplora.blogspot.com/2017/12/complete-skincare-with-skinfood.html",
}

// fetchedPassage is the durable, human-readable record of what this
// crawl actually produced - separate from the binary segment, so the
// exact provenance of every indexed passage (which URL, when) stays
// auditable without decoding the segment format.
type fetchedPassage struct {
	Text      string    `json:"text"`
	SourceURL string    `json:"source_url"`
	FetchedAt time.Time `json:"fetched_at"`
}

func main() {
	registry := extract.NewRegistry()
	registry.Register("https://simplyemsblog.wordpress.com", extract.WordPressBlogExtractor{})
	registry.Register("https://stylexplora.blogspot.com", extract.BloggerBlogExtractor{})

	f := frontier.New(5*time.Second, time.Now) // generous, real-world politeness delay
	for _, seed := range seeds {
		u, err := url.Parse(seed)
		if err != nil {
			log.Fatalf("invalid seed URL %q: %v", seed, err)
		}
		f.Add(u.Scheme+"://"+u.Host, seed)
	}

	fetcher := fetch.New(robots.New(nil), dedup.New(1000, 0.01), fetch.Config{
		MaxAttempts:  3,
		BaseDelay:    time.Second,
		MaxDelay:     10 * time.Second,
		MaxRedirects: 5,
	})

	out := make(chan fetch.Outcome)
	go fetch.Crawl(context.Background(), f, fetcher, 1, out) // 1 worker - no reason to parallelize a 5-URL, 2-host crawl

	var passages []extract.Passage
	var records []fetchedPassage

	for o := range out {
		if o.Err != nil {
			log.Printf("skipped %s: %v", o.URL, o.Err)
			continue
		}

		finalURL, err := url.Parse(o.Result.FinalURL)
		if err != nil {
			log.Printf("skipped %s: could not parse final URL %q: %v", o.URL, o.Result.FinalURL, err)
			continue
		}
		host := finalURL.Scheme + "://" + finalURL.Host
		extractor, ok := registry.ExtractorFor(host)
		if !ok {
			log.Printf("skipped %s: no extractor registered for host %s", o.Result.FinalURL, host)
			continue
		}

		ps, err := extractor.Extract(string(o.Result.Body), o.Result.FinalURL)
		if err != nil {
			log.Printf("extraction failed for %s: %v", o.Result.FinalURL, err)
			continue
		}

		now := time.Now()
		for _, p := range ps {
			passages = append(passages, p)
			records = append(records, fetchedPassage{Text: p.Text, SourceURL: p.SourceURL, FetchedAt: now})
		}
		log.Printf("extracted %d passages from %s", len(ps), o.Result.FinalURL)
	}

	log.Printf("total: %d real passages from %d seed URLs", len(passages), len(seeds))
	if len(passages) == 0 {
		log.Fatal("no passages extracted - refusing to write an empty segment")
	}

	if err := os.MkdirAll("data/real", 0o755); err != nil {
		log.Fatalf("creating data/real: %v", err)
	}

	jsonlFile, err := os.Create("data/real/passages.jsonl")
	if err != nil {
		log.Fatalf("creating passages.jsonl: %v", err)
	}
	enc := json.NewEncoder(jsonlFile)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			log.Fatalf("writing passages.jsonl: %v", err)
		}
	}
	jsonlFile.Close()

	if err := diskindex.BuildSegment(passages, "data/real/real.seg"); err != nil {
		log.Fatalf("building segment: %v", err)
	}

	log.Printf("wrote data/real/passages.jsonl and data/real/real.seg")
}
