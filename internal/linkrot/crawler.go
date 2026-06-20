package linkrot

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// Options configures a crawl.
type Options struct {
	Concurrency  int           // max simultaneous in-flight requests
	MaxDepth     int           // how many link-hops to crawl from the start page (same host)
	SameHostOnly bool          // when true, external links are skipped (not checked)
	Timeout      time.Duration // per-request timeout
	UserAgent    string
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() Options {
	return Options{
		Concurrency:  16,
		MaxDepth:     3,
		SameHostOnly: false,
		Timeout:      15 * time.Second,
		UserAgent:    "linkrot/1.0 (+https://github.com/andreaisabelmontana/linkrot)",
	}
}

// Crawler discovers pages on the start host and checks every link they reference.
type Crawler struct {
	opts   Options
	client *http.Client
	chk    checker
}

// New builds a Crawler. The HTTP client caps redirects and shares a connection pool.
func New(opts Options) *Crawler {
	if opts.Concurrency < 1 {
		opts.Concurrency = 1
	}
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}
	return &Crawler{
		opts:   opts,
		client: client,
		chk:    checker{client: client, userAgent: opts.UserAgent, timeout: opts.Timeout},
	}
}

// Run crawls the site rooted at startURL and returns the check result for every referenced link.
func (c *Crawler) Run(ctx context.Context, startURL string) ([]Result, error) {
	start, err := url.Parse(strings.TrimSpace(startURL))
	if err != nil {
		return nil, fmt.Errorf("invalid start URL: %w", err)
	}
	if start.Scheme == "" {
		start.Scheme = "https"
	}
	if start.Host == "" {
		return nil, fmt.Errorf("start URL has no host: %q", startURL)
	}
	refs := c.crawl(ctx, start)
	return c.checkAll(ctx, start, refs), nil
}

// crawl performs a bounded-concurrency BFS over same-host HTML pages, recording for every
// referenced URL the set of pages that link to it.
func (c *Crawler) crawl(ctx context.Context, start *url.URL) map[string]map[string]struct{} {
	type job struct {
		u     *url.URL
		depth int
	}

	var mu sync.Mutex
	refs := map[string]map[string]struct{}{}
	visited := map[string]struct{}{}

	sem := make(chan struct{}, c.opts.Concurrency)
	var wg sync.WaitGroup

	var crawlPage func(j job)
	crawlPage = func(j job) {
		defer wg.Done()
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		case <-ctx.Done():
			return
		}

		page := j.u.String()
		body, err := c.fetch(ctx, page)
		if err != nil || body == "" {
			return
		}
		for _, raw := range extractLinks(strings.NewReader(body)) {
			abs := resolve(j.u, raw)
			if abs == nil {
				continue
			}
			target := abs.String()

			mu.Lock()
			if refs[target] == nil {
				refs[target] = map[string]struct{}{}
			}
			refs[target][page] = struct{}{}
			_, seen := visited[target]
			descend := j.depth < c.opts.MaxDepth && sameHost(start, abs) && !seen && crawlable(abs)
			if descend {
				visited[target] = struct{}{}
			}
			mu.Unlock()

			if descend {
				wg.Add(1)
				go crawlPage(job{u: abs, depth: j.depth + 1})
			}
		}
	}

	mu.Lock()
	visited[start.String()] = struct{}{}
	mu.Unlock()
	wg.Add(1)
	go crawlPage(job{u: start, depth: 0})
	wg.Wait()
	return refs
}

// fetch GETs a page and returns its body, but only when it is HTML worth parsing.
func (c *Crawler) fetch(ctx context.Context, u string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.opts.UserAgent)
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "html") {
		return "", nil // not HTML — nothing to extract, and we still checked it separately
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20)) // 5 MB cap
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// checkAll verifies every unique referenced URL concurrently and returns results worst-first.
func (c *Crawler) checkAll(ctx context.Context, start *url.URL, refs map[string]map[string]struct{}) []Result {
	urls := make([]string, 0, len(refs))
	for u := range refs {
		urls = append(urls, u)
	}

	results := make([]Result, len(urls))
	sem := make(chan struct{}, c.opts.Concurrency)
	var wg sync.WaitGroup
	for i, u := range urls {
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			results[i] = c.checkOne(ctx, start, u, sortedKeys(refs[u]))
		}(i, u)
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		if (results[i].Status == StatusBroken) != (results[j].Status == StatusBroken) {
			return results[i].Status == StatusBroken
		}
		return results[i].URL < results[j].URL
	})
	return results
}

func (c *Crawler) checkOne(ctx context.Context, start *url.URL, rawURL string, srcs []string) Result {
	u, err := url.Parse(rawURL)
	if err != nil {
		return Result{URL: rawURL, Status: StatusSkipped, Error: "unparseable", Refs: srcs}
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return Result{URL: rawURL, Status: StatusSkipped, Refs: srcs} // mailto:, tel:, data:, …
	}
	if c.opts.SameHostOnly && !sameHost(start, u) {
		return Result{URL: rawURL, Status: StatusSkipped, Refs: srcs}
	}
	code, err := c.chk.check(ctx, rawURL)
	r := Result{URL: rawURL, StatusCode: code, Refs: srcs}
	switch {
	case err != nil:
		r.Status = StatusBroken
		r.Error = err.Error()
	case code >= 400:
		r.Status = StatusBroken
	default:
		r.Status = StatusOK
	}
	return r
}

// --- helpers ---

func resolve(base *url.URL, ref string) *url.URL {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.HasPrefix(ref, "#") {
		return nil
	}
	u, err := url.Parse(ref)
	if err != nil {
		return nil
	}
	abs := base.ResolveReference(u)
	abs.Fragment = "" // a #fragment doesn't change whether a URL resolves
	return abs
}

func sameHost(a, b *url.URL) bool { return strings.EqualFold(a.Hostname(), b.Hostname()) }

// crawlable reports whether a same-host URL is worth fetching as an HTML page (skip obvious assets).
func crawlable(u *url.URL) bool {
	ext := strings.ToLower(u.Path)
	for _, a := range []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".css", ".js", ".pdf",
		".zip", ".ico", ".webp", ".mp4", ".woff", ".woff2", ".ttf"} {
		if strings.HasSuffix(ext, a) {
			return false
		}
	}
	return true
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
