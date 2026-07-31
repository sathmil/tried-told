// Package fetch retrieves a single URL politely: checks dedup and
// robots.txt, retries transient failures with exponential backoff, and
// follows redirects manually rather than trusting net/http to do it, per
// docs/design/10-fetch-loop.md.
package fetch

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"triedandtold/internal/normalize"
	"triedandtold/internal/robots"
	"triedandtold/internal/wal"
)

// ContentRecord is one archived fetch result. If a Fetcher has a content
// log configured (WithContentLog), one of these is durably appended for
// every successful fetch - raw content storage, separate from the
// frontier/dedup scheduling logs, per docs/design/11-persistent-frontier.md.
type ContentRecord struct {
	URL         string    `json:"url"`
	FetchedAt   time.Time `json:"fetched_at"`
	StatusCode  int       `json:"status_code"`
	ContentHash string    `json:"content_hash"` // hex-encoded sha256
	Body        string    `json:"body"`
}

// ErrAlreadySeen means this URL (or a redirect target reached while
// fetching it) was already recorded as seen by the dedup registry.
var ErrAlreadySeen = errors.New("fetch: url already seen")

// ErrDisallowed means robots.txt forbids fetching this URL.
var ErrDisallowed = errors.New("fetch: disallowed by robots.txt")

// Deduper is the dedup dependency Fetcher needs. *dedup.Registry satisfies
// this directly; *crawlstate.PersistentRegistry also satisfies it, so a
// crash-resumable registry can be substituted in with no other changes to
// Fetcher - see docs/design/11-persistent-frontier.md.
type Deduper interface {
	SeenOrAdd(url string) bool
}

// Config holds the fetcher's tunable behavior.
type Config struct {
	MaxAttempts  int           // total attempts per URL, including the first
	BaseDelay    time.Duration // delay before the first retry
	MaxDelay     time.Duration // backoff cap
	MaxRedirects int           // safety cap on manual redirect-following depth
}

// Result is a successfully fetched page.
type Result struct {
	FinalURL    string // after normalization and following any redirects
	StatusCode  int
	Body        []byte
	ContentHash [32]byte // sha256 of Body, for future incremental-recrawl detection
}

// Fetcher retrieves one URL at a time. It does not schedule politeness
// across hosts - that's the Frontier's job; Fetcher is what a worker calls
// once the Frontier has said a URL is ready.
type Fetcher struct {
	client     *http.Client
	robots     *robots.Checker
	dedup      Deduper
	cfg        Config
	contentLog *wal.Log[ContentRecord]
}

// New creates a Fetcher. The returned HTTP client deliberately does not
// follow redirects automatically - see docs/design/10-fetch-loop.md for why.
func New(robots *robots.Checker, dedup Deduper, cfg Config) *Fetcher {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return &Fetcher{client: client, robots: robots, dedup: dedup, cfg: cfg}
}

// WithContentLog configures f to durably archive every successful fetch's
// body to log. Returns f so it can be chained onto New.
func (f *Fetcher) WithContentLog(log *wal.Log[ContentRecord]) *Fetcher {
	f.contentLog = log
	return f
}

// Fetch retrieves rawURL. Each hop (the original URL, and any redirect
// targets) is independently normalized, checked against dedup and
// robots.txt, and retried with backoff on transient failure.
func (f *Fetcher) Fetch(rawURL string) (*Result, error) {
	current := rawURL

	for hop := 0; hop <= f.cfg.MaxRedirects; hop++ {
		normalized, err := normalize.URL(current)
		if err != nil {
			return nil, fmt.Errorf("fetch: normalizing %q: %w", current, err)
		}

		if f.dedup.SeenOrAdd(normalized) {
			return nil, ErrAlreadySeen
		}

		allowed, err := f.robots.Allowed(normalized)
		if err != nil {
			return nil, fmt.Errorf("fetch: checking robots.txt for %q: %w", normalized, err)
		}
		if !allowed {
			return nil, ErrDisallowed
		}

		resp, err := f.fetchWithRetry(normalized)
		if err != nil {
			return nil, err
		}

		if isRedirect(resp.StatusCode) {
			location := resp.Header.Get("Location")
			resp.Body.Close()
			if location == "" {
				return nil, fmt.Errorf("fetch: redirect from %q had no Location header", normalized)
			}
			next, err := resolveRedirect(normalized, location)
			if err != nil {
				return nil, fmt.Errorf("fetch: resolving redirect target %q: %w", location, err)
			}
			current = next
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("fetch: reading body of %q: %w", normalized, err)
		}

		hash := sha256.Sum256(body)

		if f.contentLog != nil {
			if err := f.contentLog.Append(ContentRecord{
				URL:         normalized,
				FetchedAt:   time.Now(),
				StatusCode:  resp.StatusCode,
				ContentHash: hex.EncodeToString(hash[:]),
				Body:        string(body),
			}); err != nil {
				return nil, fmt.Errorf("fetch: archiving content for %q: %w", normalized, err)
			}
		}

		return &Result{
			FinalURL:    normalized,
			StatusCode:  resp.StatusCode,
			Body:        body,
			ContentHash: hash,
		}, nil
	}

	return nil, fmt.Errorf("fetch: exceeded %d redirects starting from %q", f.cfg.MaxRedirects, rawURL)
}

// fetchWithRetry retries 5xx, 429, and network-level failures with
// exponential backoff; any other outcome (including 4xx other than 429) is
// returned immediately, retried or not.
func (f *Fetcher) fetchWithRetry(url string) (*http.Response, error) {
	delay := f.cfg.BaseDelay
	var lastErr error

	for attempt := 0; attempt < f.cfg.MaxAttempts; attempt++ {
		req, reqErr := http.NewRequest(http.MethodGet, url, nil)
		if reqErr != nil {
			return nil, reqErr
		}
		// Identifies this crawler to the server it's fetching from - the
		// same name robots.Checker uses to match robots.txt rules, so a
		// site operator inspecting their own access logs can actually tell
		// which requests were this bot, not just an anonymous Go client.
		req.Header.Set("User-Agent", robots.UserAgent)

		resp, err := f.client.Do(req)
		if err == nil && !shouldRetryStatus(resp.StatusCode) {
			return resp, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			resp.Body.Close()
		}

		if attempt < f.cfg.MaxAttempts-1 {
			time.Sleep(delay)
			delay *= 2
			if delay > f.cfg.MaxDelay {
				delay = f.cfg.MaxDelay
			}
		}
	}

	return nil, fmt.Errorf("fetch: %q failed after %d attempts: %w", url, f.cfg.MaxAttempts, lastErr)
}

func shouldRetryStatus(status int) bool {
	return status == http.StatusTooManyRequests || (status >= 500 && status < 600)
}

func isRedirect(status int) bool {
	return status >= 300 && status < 400
}

// resolveRedirect resolves a Location header (absolute or relative) against
// the URL it was returned for.
func resolveRedirect(base, location string) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(ref).String(), nil
}
