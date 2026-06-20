package linkrot

import (
	"io"
	"strings"

	"golang.org/x/net/html"
)

// linkAttr maps an element name to the attribute that carries a URL.
var linkAttr = map[string]string{
	"a":      "href",
	"link":   "href",
	"img":    "src",
	"script": "src",
	"iframe": "src",
	"source": "src",
	"audio":  "src",
	"video":  "src",
}

// extractLinks tokenises HTML and returns every raw (possibly relative) URL it references.
// It is tolerant of malformed markup — the tokeniser never errors on bad HTML, only on EOF.
func extractLinks(r io.Reader) []string {
	var out []string
	z := html.NewTokenizer(r)
	for {
		switch z.Next() {
		case html.ErrorToken:
			return out
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			attr, ok := linkAttr[string(name)]
			if !ok || !hasAttr {
				continue
			}
			for {
				k, v, more := z.TagAttr()
				if string(k) == attr {
					if u := strings.TrimSpace(string(v)); u != "" {
						out = append(out, u)
					}
				}
				if !more {
					break
				}
			}
		}
	}
}
