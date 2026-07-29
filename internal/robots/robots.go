// Package robots checks URLs against a host's robots.txt, per
// docs/design/09-robots-txt.md.
package robots

import (
	"io"
	"net/http"
	"net/url"
	"sync"

	"github.com/jimsmart/grobotstxt"
)

// UserAgent identifies this crawler when fetching robots.txt and when
// matching User-agent rules within it.
const UserAgent = "TriedAndToldBot/0.1"

// outcome is what gets cached per host.
type outcome struct {
	body        string // the actual robots.txt body, if fetched successfully
	allowAll    bool   // 404: no robots.txt means no stated restrictions
	disallowAll bool   // fetch failed: fail closed until a fetch succeeds
}

// Checker fetches and caches robots.txt per host, and answers whether a
// given URL may be fetched.
type Checker struct {
	client *http.Client
	mu     sync.Mutex
	cache  map[string]outcome
}

// New creates a Checker. A nil client uses http.DefaultClient.
func New(client *http.Client) *Checker {
	if client == nil {
		client = http.DefaultClient
	}
	return &Checker{client: client, cache: make(map[string]outcome)}
}

// Allowed reports whether rawURL may be fetched under its host's
// robots.txt. A fetch failure fails closed (not allowed) for this call, but
// is never cached - the next call retries the fetch fresh rather than
// being permanently stuck, leaving actual retry timing to the caller.
func (c *Checker) Allowed(rawURL string) (bool, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false, err
	}
	host := u.Scheme + "://" + u.Host

	o := c.outcomeFor(host)

	switch {
	case o.allowAll:
		return true, nil
	case o.disallowAll:
		return false, nil
	default:
		return grobotstxt.AgentAllowed(o.body, UserAgent, rawURL), nil
	}
}

func (c *Checker) outcomeFor(host string) outcome {
	c.mu.Lock()
	if cached, ok := c.cache[host]; ok {
		c.mu.Unlock()
		return cached
	}
	c.mu.Unlock()

	o := c.fetch(host)

	if !o.disallowAll {
		c.mu.Lock()
		c.cache[host] = o
		c.mu.Unlock()
	}

	return o
}

func (c *Checker) fetch(host string) outcome {
	resp, err := c.client.Get(host + "/robots.txt")
	if err != nil {
		return outcome{disallowAll: true} // couldn't even connect
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return outcome{allowAll: true}
	}
	if resp.StatusCode != http.StatusOK {
		return outcome{disallowAll: true} // server error / unexpected status
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return outcome{disallowAll: true}
	}
	return outcome{body: string(body)}
}
