package linkrot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func mustParse(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return u
}

func TestCrawlFindsBrokenLinks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// ServeMux's "/" is a catch-all; only serve the real root, 404 everything else
		// (so /missing and /also-missing genuinely 404).
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`
			<a href="/good">good</a>
			<a href="/missing">missing</a>
			<a href="/page2">page two</a>
			<img src="/img.png">`))
	})
	mux.HandleFunc("/good", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/img.png", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/page2", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<a href="/also-missing">deep</a>`))
	})
	// /missing and /also-missing are unregistered -> 404.
	srv := httptest.NewServer(mux)
	defer srv.Close()

	opts := DefaultOptions()
	opts.Timeout = 5 * time.Second
	results, err := New(opts).Run(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	status := map[string]Status{}
	for _, r := range results {
		status[r.URL] = r.Status
	}
	if status[srv.URL+"/missing"] != StatusBroken {
		t.Errorf("/missing should be broken, got %q", status[srv.URL+"/missing"])
	}
	if status[srv.URL+"/also-missing"] != StatusBroken {
		t.Errorf("/also-missing (depth 2) should be broken, got %q", status[srv.URL+"/also-missing"])
	}
	if status[srv.URL+"/good"] != StatusOK {
		t.Errorf("/good should be ok, got %q", status[srv.URL+"/good"])
	}
	if status[srv.URL+"/img.png"] != StatusOK {
		t.Errorf("/img.png should be ok, got %q", status[srv.URL+"/img.png"])
	}

	s := Summarize(results)
	if s.Broken < 2 {
		t.Errorf("expected >=2 broken, got %d (%+v)", s.Broken, s)
	}
	// Broken results must sort first.
	if len(results) > 0 && results[0].Status != StatusBroken {
		t.Errorf("expected broken-first ordering, first was %q", results[0].Status)
	}
}

func TestSameHostOnlySkipsExternal(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<a href="https://example.invalid/x">ext</a><a href="/local">loc</a>`))
	})
	mux.HandleFunc("/local", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	opts := DefaultOptions()
	opts.SameHostOnly = true
	opts.Timeout = 3 * time.Second
	results, err := New(opts).Run(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.URL == "https://example.invalid/x" && r.Status != StatusSkipped {
			t.Errorf("external link should be skipped in internal-only mode, got %q", r.Status)
		}
	}
}

func TestRunRejectsURLWithoutHost(t *testing.T) {
	if _, err := New(DefaultOptions()).Run(context.Background(), "http://"); err == nil {
		t.Error("expected error for URL without host")
	}
}
