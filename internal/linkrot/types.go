// Package linkrot crawls a website and concurrently verifies that every link and asset it
// references actually resolves — a fast, dependency-light broken-link checker.
package linkrot

// Status is the verdict for a single checked URL.
type Status string

const (
	StatusOK      Status = "ok"      // reachable (2xx/3xx)
	StatusBroken  Status = "broken"  // 4xx/5xx or transport error
	StatusSkipped Status = "skipped" // non-HTTP scheme, or excluded by options
)

// Result is the outcome of checking one URL, including which pages referenced it.
type Result struct {
	URL        string   `json:"url"`
	Status     Status   `json:"status"`
	StatusCode int      `json:"status_code,omitempty"`
	Error      string   `json:"error,omitempty"`
	Refs       []string `json:"referenced_by,omitempty"`
}

// OK reports whether the URL resolved successfully.
func (r Result) OK() bool { return r.Status == StatusOK }
