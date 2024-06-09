package main

import (
	"context"
	"fmt"
	"github.com/bcampbell/scrapeomat/extract"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
	"net/http"
	neturl "net/url"
	"os"
	"strings"
)

// DiscoverArticles crawls startURL looking for article urls.
// ctx can be cancelled to abort the operation.
func DiscoverArticles(ctx context.Context, client *http.Client, startURL string) ([]string, error) {
	d := Discoverer{
		MaxDepth: 1,
		ctx:      ctx,
		visited:  map[string]struct{}{},
	}
	artLinks, err := d.crawl(0, startURL, client)
	if err != nil {
		return nil, err
	}

	out := []string{}
	for l, _ := range artLinks {
		out = append(out, l)
	}
	return out, nil
}

type Discoverer struct {
	MaxDepth       int
	httpErrorCount int
	ctx            context.Context
	visited        map[string]struct{}
}

func (d *Discoverer) crawl(depth int, url string, client *http.Client) (map[string]struct{}, error) {
	base, err := neturl.Parse(url)
	if err != nil {
		return nil, err
	}

	req, err := buildRequest(d.ctx, url)

	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Allow some http errors.
	if resp.StatusCode != 200 {
		d.httpErrorCount++
		threshold := len(d.visited) / 8
		if threshold < 10 {
			threshold = 10
		}
		if d.httpErrorCount > threshold {
			return nil, fmt.Errorf("HTTP error: %s (%s)", resp.Status, url)
		}
		// suppress error
		fmt.Fprintf(os.Stderr, "HTTP error (%d/%d): %s (%s)\n", d.httpErrorCount, threshold, resp.Status, url)
		return map[string]struct{}{}, nil
	}

	cached := false
	// was cached locally?
	if resp.Header.Get("X-From-Cache") != "" {
		cached = true
	}

	d.visited[url] = struct{}{}

	root, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	artLinks, navLinks, err := d.extractLinks(root, base)
	if err != nil {
		return nil, err
	}

	// Figure out which nav links we've not already visited.
	newNavLinks := map[string]struct{}{}
	for l, _ := range navLinks {
		if _, seen := d.visited[l]; !seen {
			newNavLinks[l] = struct{}{}
		}
	}

	// Aid potential garbage collection
	resp = nil
	root = nil

	foo := ""
	if cached {
		foo = "cached"
	}
	fmt.Printf("%s (depth=%d %s) %d artlinks, %d navlinks (%d new)\n", url, depth, foo, len(artLinks), len(navLinks), len(newNavLinks))
	if depth < d.MaxDepth {
		// Recurse
		for navURL, _ := range newNavLinks {
			newArtLinks, err := d.crawl(depth+1, navURL, client)
			if err != nil {
				return nil, err
			}
			// Add new art links to what we've already got.
			for l, _ := range newArtLinks {
				artLinks[l] = struct{}{}
			}
		}
	}
	return artLinks, nil
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

func getAbsHref(n *html.Node, base *neturl.URL) *neturl.URL {
	href := getAttr(n, "href")
	if href == "" {
		return nil
	}

	u, err := neturl.Parse(href)
	if err != nil {
		return nil
	}

	return base.ResolveReference(u)
}

var (
	//selNavLink = cascadia.MustCompile(`nav a`)
	selLink = cascadia.MustCompile(`a`)
	//selRelCanonical = cascadia.MustCompile(`link[rel="canonical"]`)
)

func (d *Discoverer) extractLinks(root *html.Node, base *neturl.URL) (map[string]struct{}, map[string]struct{}, error) {
	// NOTES:
	// "nav a" is a good navlink selector on a lot of sites
	navLinks := map[string]struct{}{}
	artLinks := map[string]struct{}{}

	// Look for nav blocks first.
	for _, a := range selLink.MatchAll(root) {

		link := getAbsHref(a, base)
		if link == nil {
			// TODO: show warning?
			continue
		}
		link.Fragment = ""
		link.RawQuery = ""

		// discard if not on same site
		if link.Hostname() != base.Hostname() {
			continue
		}

		// Analyse slug
		slug := extract.SlugFromURL(link)
		if slug == "" {
			continue
		}
		slug = strings.ReplaceAll(slug, "_", "-")
		nParts := len(strings.Split(slug, "-"))

		if nParts < 4 {
			// looks suitable as a navigation link
			navLinks[link.String()] = struct{}{}
		}

		if nParts >= 2 {
			artLinks[link.String()] = struct{}{}
		}

	}
	return artLinks, navLinks, nil
}
