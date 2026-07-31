// Command crawl runs a real crawl bounded to a small set of individually
// vetted real sites - see docs/design/33-real-crawl.md for how each
// platform was checked (robots.txt fetched and read directly, terms of
// service checked for any scraping restriction) before being trusted at
// all, and docs/design/37-crawl-link-discovery.md for the decision to
// let this crawl follow links.
//
// Unlike the original version of this command, this is no longer a
// fixed, hand-curated URL list: listingSeeds are fetched purely to
// discover further post links, which get enqueued and crawled too. That
// discovery is bounded in every direction that matters - discovered
// links are restricted to hosts already registered in registry (no new,
// unvetted site is ever reached this way), every discovered URL still
// gets the exact same per-URL robots.txt check as everything else via
// fetch.Fetcher, and maxDiscoveredPosts caps the total so a single run
// stays a deliberate, bounded expansion rather than an open-ended one.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"triedandtold/internal/dedup"
	"triedandtold/internal/diskindex"
	"triedandtold/internal/extract"
	"triedandtold/internal/fetch"
	"triedandtold/internal/frontier"
	"triedandtold/internal/robots"
)

const (
	passagesPath = "data/real/passages.jsonl"
	segmentPath  = "data/real/real.seg"

	// maxDiscoveredPosts caps how many new post pages a single run will
	// fetch beyond what's already in passagesPath - a deliberate bound,
	// not a technical limit, so growing the corpus stays a conscious,
	// repeatable decision rather than "crawl everything available."
	maxDiscoveredPosts = 40
)

// listingSeeds are fetched only to discover further post links from -
// never passage-extracted themselves (a category/label index page has
// no review content of its own). Each host here was already vetted in
// docs/design/33-real-crawl.md.
var listingSeeds = []string{
	"https://simplyemsblog.wordpress.com/category/beauty/",
	"https://simplyemsblog.wordpress.com/category/beauty/page/2/",
	"https://simplyemsblog.wordpress.com/category/beauty/page/3/",
	"https://stylexplora.blogspot.com/search/label/skincare",
	"https://stylexplora.blogspot.com/search/label/acne%20skincare",
	"https://stylexplora.blogspot.com/search/label/natural%20skincare",
	"https://stylexplora.blogspot.com/search/label/beauty",
}

// postURLPattern matches an individual post's URL path on each vetted
// host, distinguishing real post links (found on a listing page
// alongside plenty of navigation/widget/pagination links) from
// everything else on that page.
var postURLPattern = map[string]*regexp.Regexp{
	"https://simplyemsblog.wordpress.com": regexp.MustCompile(`^/\d{4}/\d{2}/\d{2}/[a-z0-9-]+/?$`),
	"https://stylexplora.blogspot.com":    regexp.MustCompile(`^/\d{4}/\d{2}/[a-z0-9-]+\.html$`),
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

	existingRecords, alreadyKnown := loadExistingRecords(passagesPath)
	log.Printf("starting from %d already-crawled passages", len(existingRecords))

	f := frontier.New(5*time.Second, time.Now) // generous, real-world politeness delay
	isListing := make(map[string]bool, len(listingSeeds))
	for _, seed := range listingSeeds {
		u, err := url.Parse(seed)
		if err != nil {
			log.Fatalf("invalid listing seed %q: %v", seed, err)
		}
		f.Add(u.Scheme+"://"+u.Host, seed)
		isListing[seed] = true
	}

	fetcher := fetch.New(robots.New(nil), dedup.New(1000, 0.01), fetch.Config{
		MaxAttempts:  3,
		BaseDelay:    time.Second,
		MaxDelay:     10 * time.Second,
		MaxRedirects: 5,
	})

	out := make(chan fetch.Outcome)
	go fetch.Crawl(context.Background(), f, fetcher, 1, out) // 1 worker - a real crawl has no reason to hit multiple hosts concurrently here

	passages := make([]extract.Passage, 0, len(existingRecords))
	for _, r := range existingRecords {
		passages = append(passages, extract.Passage{Text: r.Text, SourceURL: r.SourceURL})
	}
	records := existingRecords

	enqueued := make(map[string]bool) // posts already queued this run, discovered or not, so no listing page double-queues one
	discovered := 0

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
		body := string(o.Result.Body)

		if isListing[o.URL] || isListing[o.Result.FinalURL] {
			found := 0
			for _, link := range discoverPostLinks(body, o.Result.FinalURL) {
				if alreadyKnown[link] || enqueued[link] {
					continue
				}
				if discovered >= maxDiscoveredPosts {
					break
				}
				u, err := url.Parse(link)
				if err != nil {
					continue
				}
				f.Add(u.Scheme+"://"+u.Host, link)
				enqueued[link] = true
				discovered++
				found++
			}
			log.Printf("listing page %s: queued %d new posts (%d/%d total)", o.Result.FinalURL, found, discovered, maxDiscoveredPosts)
			continue
		}

		if alreadyKnown[o.Result.FinalURL] {
			continue // already crawled in a previous run
		}

		extractor, ok := registry.ExtractorFor(host)
		if !ok {
			log.Printf("skipped %s: no extractor registered for host %s", o.Result.FinalURL, host)
			continue
		}
		ps, err := extractor.Extract(body, o.Result.FinalURL)
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

	log.Printf("total: %d passages (%d newly crawled this run)", len(passages), len(passages)-len(existingRecords))
	if len(passages) == 0 {
		log.Fatal("no passages at all - refusing to write an empty segment")
	}

	if err := os.MkdirAll("data/real", 0o755); err != nil {
		log.Fatalf("creating data/real: %v", err)
	}

	jsonlFile, err := os.Create(passagesPath)
	if err != nil {
		log.Fatalf("creating %s: %v", passagesPath, err)
	}
	enc := json.NewEncoder(jsonlFile)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			log.Fatalf("writing %s: %v", passagesPath, err)
		}
	}
	jsonlFile.Close()

	if err := diskindex.BuildSegment(passages, segmentPath); err != nil {
		log.Fatalf("building segment: %v", err)
	}

	log.Printf("wrote %s and %s", passagesPath, segmentPath)
}

// loadExistingRecords reads a previous run's output, if any, so a new
// run extends the corpus instead of starting over or re-fetching pages
// it already has. alreadyKnown is keyed by page URL (SourceURL), not
// passage ID, since the check that matters here is "has this page
// already been crawled," before any passage from it even exists.
func loadExistingRecords(path string) ([]fetchedPassage, map[string]bool) {
	known := make(map[string]bool)

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, known
	}
	if err != nil {
		log.Fatalf("reading existing %s: %v", path, err)
	}
	defer f.Close()

	var records []fetchedPassage
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var rec fetchedPassage
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			log.Fatalf("parsing existing %s: %v", path, err)
		}
		records = append(records, rec)
		known[rec.SourceURL] = true
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("reading existing %s: %v", path, err)
	}
	return records, known
}

// discoverPostLinks finds every link on a listing page's HTML that
// points to an individual post on the *same* host the listing page
// itself was fetched from, matching that host's known post-URL shape
// (postURLPattern) - link text pointing anywhere else (navigation,
// other widgets, a different host entirely) is not a post and is
// ignored, not followed.
func discoverPostLinks(html, pageURL string) []string {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}
	host := base.Scheme + "://" + base.Host
	pattern, ok := postURLPattern[host]
	if !ok {
		return nil
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var links []string
	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		ref, err := url.Parse(href)
		if err != nil {
			return
		}
		abs := base.ResolveReference(ref)
		if abs.Scheme+"://"+abs.Host != host {
			return
		}
		if !pattern.MatchString(abs.Path) {
			return
		}
		normalized := host + abs.Path
		if !seen[normalized] {
			seen[normalized] = true
			links = append(links, normalized)
		}
	})
	return links
}
