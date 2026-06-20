package linkrot

import (
	"context"
	"io"
	"net/http"
	"time"
)

// checker verifies that a single URL is reachable. It prefers a cheap HEAD request and falls
// back to GET when a server rejects HEAD (many do with 403/405).
type checker struct {
	client    *http.Client
	userAgent string
	timeout   time.Duration
}

func (c checker) check(ctx context.Context, rawURL string) (int, error) {
	code, err := c.do(ctx, http.MethodHead, rawURL)
	if err == nil && code < 400 {
		return code, nil
	}
	// HEAD unsupported / blocked / errored — retry with GET before declaring it broken.
	if err != nil || code == http.StatusMethodNotAllowed || code == http.StatusForbidden ||
		code == http.StatusNotImplemented {
		return c.do(ctx, http.MethodGet, rawURL)
	}
	return code, nil
}

func (c checker) do(ctx context.Context, method, rawURL string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	// Drain a little so the connection can be reused; cap it so GET on a huge file is cheap.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 32<<10))
	return resp.StatusCode, nil
}
