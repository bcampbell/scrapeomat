package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/andybalholm/cascadia"
	"github.com/bcampbell/scrapeomat/extract"
	"github.com/bcampbell/scrapeomat/store"
	"golang.org/x/net/html"
	"net/http"
	neturl "net/url"
	"strings"
	"time"
)

type Discoverer struct {
	MaxDepth  int
	numErrors int
	visited   map[string]struct{}
	ErrLog    store.Logger
	InfoLog   store.Logger
	DebugLog  store.Logger
	StartTime time.Time
}

// DiscoverArticles crawls startURL looking for article urls.
// ctx can be cancelled to abort the operation.
func (d *Discoverer) DiscoverArticles(ctx context.Context, client *http.Client, startURL string) ([]string, error) {
	d.StartTime = time.Now()
	d.InfoLog.Printf("start discovery at %s\n", startURL)
	artLinks, err := d.crawl(ctx, 0, startURL, client)
	if err != nil {
		return nil, err
	}

	out := []string{}
	for l, _ := range artLinks {
		out = append(out, l)
	}

	elapsed := time.Now().Sub(d.StartTime)
	d.InfoLog.Printf("Discovery yielded %d article (visited %d pages, with %d errors, took %v)\n", len(out), len(d.visited), d.numErrors, elapsed)
	return out, nil
}

func (d *Discoverer) crawl(ctx context.Context, depth int, url string, client *http.Client) (map[string]struct{}, error) {
	base, err := neturl.Parse(url)
	if err != nil {
		return nil, err
	}

	req, err := buildRequest(ctx, url)

	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
	}

	// Allow some errors.
	if err != nil || (err == nil && resp.StatusCode != 200) {
		if errors.Is(err, context.Canceled) {
			return nil, err // Bail out immediately.
		}
		d.numErrors++
		threshold := len(d.visited) / 8
		if threshold < 10 {
			threshold = 10
		}
		if err == nil {
			// It's an http error.
			d.ErrLog.Printf("(err %d/%d) HTTP %s %s\n", d.numErrors, threshold, resp.Status, url)
		} else {
			// some other error
			d.ErrLog.Printf("(err %d/%d) %s\n", d.numErrors, threshold, err)
		}
		if d.numErrors > threshold {
			return nil, fmt.Errorf("Too many errors during discovery")
		}

		if err != nil {
			// keep going...
			return map[string]struct{}{}, nil
		}
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
		foo = " (cached)"
	}
	d.DebugLog.Printf("depth=%d%s %s - %d artlinks, %d navlinks (%d new)\n", depth, foo, url, len(artLinks), len(navLinks), len(newNavLinks))
	if depth < d.MaxDepth {
		// Recurse
		for navURL, _ := range newNavLinks {
			newArtLinks, err := d.crawl(ctx, depth+1, navURL, client)
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
