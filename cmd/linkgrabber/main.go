package main

import (
	"flag"
	"fmt"
	"github.com/andybalholm/cascadia"
	"github.com/bcampbell/arts/util"
	"golang.org/x/net/html"
	"net/http"
	"net/url"
	"os"
)

var opts struct {
	linkSel   string
	followSel string
	verbose   bool
}

func main() {
	flag.Usage = func() {

		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "%s [OPTIONS] URL(s)...\n", os.Args[0])
		fmt.Fprintf(os.Stderr, `
Scans the pages at the given URLs and dumps all the links out to stdout.

Input URLs can be absolute or relative - relative links will be
considered relative to the previous URL in the list.
eg:
   http://pseudopolisherald.com/ /politics /local /hubwards
is just fine.

`)
		flag.PrintDefaults()
	}

	flag.StringVar(&opts.linkSel, "l", "a", "css selector to find links to output")
	flag.StringVar(&opts.followSel, "f", "", "css selector of links to follow")
	flag.BoolVar(&opts.verbose, "v", false, "output extra info (on stderr)")
	flag.Parse()

	var err error
	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "ERROR: missing URL(s)\n")
		flag.Usage()
		os.Exit(1)
	}

	err = doit(flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

// expandURLs produces a list of absolute URLs from a list of (perhaps) partial
// URLs. It uses the previous URL in the list as the context for the next.
func expandURLs(origURLs []string) ([]string, error) {
	prev := &url.URL{}
	cooked := make([]string, len(origURLs))
	for i, origURL := range origURLs {
		parsed, err := prev.Parse(origURL)
		if err != nil {
			return nil, fmt.Errorf("bad URL '%s'", origURL)
		}

		if !parsed.IsAbs() {
			return nil, fmt.Errorf("URL not absolute (and can't be guessed from previous) '%s'", origURL)
		}
		prev = parsed
		cooked[i] = parsed.String()
	}
	return cooked, nil
}

func doit(urls []string) error {
	linkSel, err := cascadia.Compile(opts.linkSel)
	if err != nil {
		return fmt.Errorf("Bad link selector: %s", err)
	}

	var followSel cascadia.Selector = nil
	if opts.followSel != "" {
		followSel, err = cascadia.Compile(opts.followSel)
		if err != nil {
			return fmt.Errorf("Bad follow selector: %s", err)
		}
	}

	urls, err = expandURLs(urls)
	if err != nil {
		return err
	}

	client := &http.Client{
		Transport: util.NewPoliteTripper(),
	}

	queued := map[string]struct{}{}
	visited := map[string]struct{}{}

	for _, u := range urls {
		queued[u] = struct{}{}
	}

	errCnt := 0
	// while we have urls queued to scrape...
	for len(queued) > 0 {
		for u, _ := range queued {
			found, follow, err := doPage(client, u, linkSel, followSel)

			// shift url into visited set
			visited[u] = struct{}{}
			delete(queued, u)
			if err != nil {
				fmt.Fprintf(os.Stderr, "FAILED: %s (%s)\n", u, err)
				errCnt++
				if errCnt > 10 {
					return fmt.Errorf("Too many errors.")
				}
				continue
			}
			if opts.verbose {
				fmt.Fprintf(os.Stderr, "%s (%d,%d)\n", u, len(found), len(follow))
			}

			// output any found links
			for _, l := range found {
				fmt.Println(l)
			}

			// queue up any links we want to follow
			for _, l := range follow {
				_, got := visited[l]
				if !got {
					queued[l] = struct{}{}
				}
			}
		}
	}

	return nil
}

func doPage(client *http.Client, pageURL string, linkSel cascadia.Selector, followSel cascadia.Selector) ([]string, []string, error) {

	root, err := fetchAndParse(client, pageURL)
	if err != nil {
		return []string{}, []string{}, err
	}
	found, err := grabLinks(root, linkSel, pageURL)
	if err != nil {
		return []string{}, []string{}, err
	}

	follow := []string{}
	if followSel != nil {
		follow, err = grabLinks(root, followSel, pageURL)
		if err != nil {
			return []string{}, []string{}, err
		}
	}

	return found, follow, nil
}

func fetchAndParse(client *http.Client, u string) (*html.Node, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	// NOTE: FT.com always returns 403 if no Accept header is present.
	// Seems like a reasonable thing to send anyway...
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	// TODO: verbose flag!!!
	// fmt.Fprintf(os.Stderr, "fetch %s\n", u)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err = fmt.Errorf("HTTP code %d (%s)", resp.StatusCode, u)
		return nil, err
	}

	return html.Parse(resp.Body)
}

// GetAttr retrieved the value of an attribute on a node.
// Returns empty string if attribute doesn't exist.
func GetAttr(n *html.Node, attr string) string {
	for _, a := range n.Attr {
		if a.Key == attr {
			return a.Val
		}
	}
	return ""
}

func grabLinks(root *html.Node, linkSel cascadia.Selector, baseURL string) ([]string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	out := []string{}
	for _, a := range linkSel.MatchAll(root) {
		link, err := getAbsHref(a, u)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s BAD link: '%s'\n", baseURL, err)
			continue
		}
		out = append(out, link)
	}
	return out, nil
}

func getAbsHref(anchor *html.Node, baseURL *url.URL) (string, error) {
	h := GetAttr(anchor, "href")
	absURL, err := baseURL.Parse(h)
	if err != nil {
		return "", fmt.Errorf("bad href (%s): %s", h, err)
	}
	return absURL.String(), nil
}
