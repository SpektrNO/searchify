package extract

import (
	"context"
	"io"
	"strings"

	"golang.org/x/net/html"
)

type htmlExtractor struct{}

func (htmlExtractor) Extensions() []string { return []string{".html", ".htm"} }

func (htmlExtractor) Extract(ctx context.Context, path string, r io.Reader, size int64) (string, []string, error) {
	_ = path
	_ = size
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	doc, err := html.Parse(r)
	if err != nil {
		return "", nil, err
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "script", "style", "noscript":
				return
			case "br", "p", "div", "tr", "li", "h1", "h2", "h3", "h4", "h5", "h6", "section", "article":
				b.WriteByte('\n')
			}
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	text := collapseBlankLines(b.String())
	if text == "" {
		return "", nil, errEmptyHTML
	}
	return text, nil, nil
}

var errEmptyHTML = errString("html: no visible text")

type errString string

func (e errString) Error() string { return string(e) }
