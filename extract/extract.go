package extract

import (
	//	"fmt"
	"bytes"
	"github.com/andybalholm/cascadia"
	"github.com/dyatlov/go-opengraph/opengraph"
	readability "github.com/go-shiori/go-readability"
	//	"io"
	"golang.org/x/net/html"
	"net/http"
	"net/url"
	"regexp"
)

type ArtInfo struct {
	URL           string
	Readability   *readability.Article
	OpenGraph     *opengraph.OpenGraph
	URLHeuristics *URLHeuristics
	CanonicalURL  string
}

func Extract(pageURL *url.URL, header *http.Header, body []byte) (*ArtInfo, error) {
	parser := readability.NewParser()
	readable, err := parser.Parse(bytes.NewReader(body), pageURL)
	if err != nil {
		return nil, err
	}

	og := opengraph.NewOpenGraph()
	err = og.ProcessHTML(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	uh := SniffURL(pageURL.String())

	root, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	canonicalURL := SniffRelCanonical(root)

	return &ArtInfo{
		URL:           pageURL.String(),
		Readability:   &readable,
		OpenGraph:     og,
		URLHeuristics: uh,
		CanonicalURL:  canonicalURL,
	}, nil

	// TODO:
	// Proper schema.org NewsArticle parsing
	// Sanitise content? (see arts/tidy.go)
	// Authors
	// Section
	// Publication
}

// getAttr retrieves the value of an attribute on a node.
// Returns empty string if attribute doesn't exist.
func getAttr(n *html.Node, attr string) string {
	for _, a := range n.Attr {
		if a.Key == attr {
			return a.Val
		}
	}
	return ""
}

var (
	selRelCanonical = cascadia.MustCompile(`link[rel="canonical"]`)
	rxSlug          = regexp.MustCompile(`(?i)[a-z0-9]+(?:-[a-z0-9]+)+`)
	rxSlugDate      = regexp.MustCompile(`/\d{4}/\d{2}/\d{2}/`)
	rxNumericID     = regexp.MustCompile(`\d{5,}`)
)

type URLHeuristics struct {
	Slug      bool
	Date      bool
	NumericID bool
}

func SniffURL(u string) *URLHeuristics {
	h := &URLHeuristics{}

	h.Slug = rxSlug.MatchString(u)
	h.NumericID = rxNumericID.MatchString(u)

	// TODO: Check day/month is sensible.
	h.Date = rxSlugDate.MatchString(u)
	return h
}

func SniffRelCanonical(root *html.Node) string {
	n := selRelCanonical.MatchFirst(root)
	if n == nil {
		return ""
	}
	return getAttr(n, "href")
}
