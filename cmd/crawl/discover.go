package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/andybalholm/cascadia"
	"github.com/bcampbell/scrapeomat/extract"
	"github.com/bcampbell/scrapeomat/grab"
	"github.com/bcampbell/scrapeomat/store"
	"golang.org/x/net/html"
	neturl "net/url"
	"strings"
	"time"
)

type Discoverer struct {
	MaxDepth     int
	numErrors    int
	numCacheHits int
	visited      map[string]struct{}
	ErrLog       store.Logger
	InfoLog      store.Logger
	DebugLog     store.Logger
	StartTime    time.Time
}

// DiscoverArticles crawls startURL looking for article urls.
// ctx can be cancelled to abort the operation.
func (d *Discoverer) DiscoverArticles(ctx context.Context, grabber grab.Grabber, startURL string) ([]string, error) {
	d.StartTime = time.Now()
	d.InfoLog.Printf("Start discovery at %s\n", startURL)
	artLinks, err := d.crawl(ctx, 0, startURL, grabber)
	if err != nil {
		return nil, err
	}

	out := []string{}
	for l, _ := range artLinks {
		out = append(out, l)
	}

	elapsed := time.Now().Sub(d.StartTime)
	d.InfoLog.Printf("Discovery yielded %d articles (%d visits, %d cachehits, %d errors, took %s)\n", len(out), len(d.visited), d.numCacheHits, d.numErrors, elapsed.Truncate(time.Second))
	return out, nil
}

func (d *Discoverer) crawl(ctx context.Context, depth int, url string, grabber grab.Grabber) (map[string]struct{}, error) {
	base, err := neturl.Parse(url)
	if err != nil {
		return nil, err
	}

	header, body, err := grabber.Grab(ctx, url)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, err // Bail out immediately.
		}
		// Allow some errors.
		d.numErrors++
		threshold := len(d.visited) / 8
		if threshold < 10 {
			threshold = 10
		}
		d.ErrLog.Printf("(err %d/%d) %s\n", d.numErrors, threshold, err)
		if d.numErrors > threshold {
			return nil, fmt.Errorf("Too many errors during discovery")
		}

		// Swallow error, continue crawling.
		return map[string]struct{}{}, nil
	}

	cached := false
	// was cached locally?
	if header.Get("X-From-Cache") != "" {
		cached = true
		d.numCacheHits++
	}

	d.visited[url] = struct{}{}

	root, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	artLinks, navLinks, err := d.extractLinks(root, base)
	if err != nil {
		return nil, err
	}

	// Aid potential garbage collection
	root = nil

	// Log results from this page.
	numNewLinks := 0
	for l, _ := range navLinks {
		if _, seen := d.visited[l]; !seen {
			numNewLinks++
		}
	}
	cachedStatus := ""
	if cached {
		cachedStatus = " (cached)"
	}
	d.DebugLog.Printf("depth=%d%s %s - %d artlinks, %d navlinks (%d new)\n", depth, cachedStatus, url, len(artLinks), len(navLinks), numNewLinks)

	if depth < d.MaxDepth {
		// Recurse
		for navURL, _ := range navLinks {
			if _, seen := d.visited[navURL]; seen {
				continue // Already visited.
			}
			newArtLinks, err := d.crawl(ctx, depth+1, navURL, grabber)
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
