package extract

import (
	"bytes"
	"github.com/andybalholm/cascadia"
	"github.com/dyatlov/go-opengraph/opengraph"
	readability "github.com/go-shiori/go-readability"
	"golang.org/x/net/html"
	"net/http"
	"net/url"
	"regexp"
)

type ArtInfo struct {
	URL           *url.URL
	Readability   *readability.Article
	OpenGraph     *opengraph.OpenGraph
	URLHeuristics *URLHeuristics
	CanonicalURL  string
	Links         []string
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

	links := collectLinks(pageURL, root)

	return &ArtInfo{
		URL:           pageURL,
		Readability:   &readable,
		OpenGraph:     og,
		URLHeuristics: uh,
		CanonicalURL:  canonicalURL,
		Links:         links,
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
	selLink         = cascadia.MustCompile(`a`)
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

// SniffURL picks out some information about a url that might
// help decide if it's an article or not.
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

func collectLinks(base *url.URL, root *html.Node) []string {
	found := map[string]struct{}{}
	for _, l := range selLink.MatchAll(root) {

		href := getAttr(l, "href")
		if href == "" {
			continue
		}
		u, err := url.Parse(href)
		if err != nil {
			continue
		}
		u.Fragment = ""
		u.RawQuery = ""

		link := base.ResolveReference(u)

		found[link.String()] = struct{}{}
	}

	out := []string{}
	for link, _ := range found {
		out = append(out, link)
	}
	return out
}
