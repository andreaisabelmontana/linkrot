package linkrot

import (
	"strings"
	"testing"
)

func TestExtractLinks(t *testing.T) {
	doc := `<html><body>
		<a href="/about">about</a>
		<img src="pic.png">
		<link href="style.css" rel="stylesheet">
		<script src="/app.js"></script>
		<a href="#frag">frag</a>
		<a href="mailto:x@y.com">mail</a>
		<p>no link here</p>
	</body></html>`

	got := extractLinks(strings.NewReader(doc))
	want := []string{"/about", "pic.png", "style.css", "/app.js", "#frag", "mailto:x@y.com"}

	set := map[string]bool{}
	for _, g := range got {
		set[g] = true
	}
	if len(got) != len(want) {
		t.Fatalf("extracted %d links %v, want %d", len(got), got, len(want))
	}
	for _, w := range want {
		if !set[w] {
			t.Errorf("missing expected link %q", w)
		}
	}
}

func TestExtractHandlesMalformedHTML(t *testing.T) {
	// Unclosed tags / stray attributes must not panic or error.
	doc := `<a href="/ok"><img src=broken.png <div><a href=/two>`
	got := extractLinks(strings.NewReader(doc))
	if len(got) == 0 {
		t.Fatal("expected to extract at least one link from malformed HTML")
	}
}

func TestResolveAndSameHost(t *testing.T) {
	base := mustParse(t, "https://example.com/dir/page.html")
	abs := resolve(base, "../other.html")
	if abs == nil || abs.String() != "https://example.com/other.html" {
		t.Fatalf("resolve gave %v", abs)
	}
	if !sameHost(base, abs) {
		t.Error("expected same host")
	}
	if resolve(base, "#top") != nil {
		t.Error("pure fragment should resolve to nil")
	}
}
