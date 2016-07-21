package main

// TODO: add some error tolerance (eg wayback machine produces a server timeout 408 sometimes)
// TODO: filter out links to other domains

import (
	"flag"
	"fmt"
	"github.com/andybalholm/cascadia"
	"github.com/bcampbell/arts/util"
	"golang.org/x/net/html"
	"net/http"
	"net/url"
	"os"
	"time"
)

type Options struct {
	dayFrom, dayTo string
}

func (opts *Options) DayRange() ([]time.Time, error) {
	from, to, err := opts.parseDays()
	if err != nil {
		return nil, err
	}

	// make sure we're at start of day
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)

	out := []time.Time{}
	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		out = append(out, day)
	}
	return out, nil
}

func (opts *Options) parseDays() (time.Time, time.Time, error) {

	const dayFmt = "2006-01-02"
	z := time.Time{}

	var from, to time.Time
	var err error
	if opts.dayFrom == "" {
		return z, z, fmt.Errorf("'from' day required")
	}
	from, err = time.Parse(dayFmt, opts.dayFrom)
	if err != nil {
		return z, z, fmt.Errorf("bad 'from' day (%s)", err)
	}

	if opts.dayTo == "" {
		return z, z, fmt.Errorf("'to' day required")
	}
	to, err = time.Parse(dayFmt, opts.dayTo)
	if err != nil {
		return z, z, fmt.Errorf("bad 'to' day (%s)", err)
	}

	if to.Before(from) {
		return z, z, fmt.Errorf("bad date range ('from' is after 'to')")
	}

	return from, to, nil
}

func main() {
	flag.Usage = func() {

		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "%s [OPTIONS] URL(s)...\n", os.Args[0])
		fmt.Fprintf(os.Stderr, `
Grabs page snapshots from wayback machine for URLs over the given time
period, scans them for links, and dumps them out to stdout.


Input URLs can be absolute or relative - relative links will be
considered relative to the previous URL in the list.
eg:
   http://www.telegraph.co.uk/ /news/ /sport/ /business/
is just fine.

options:
`)
		flag.PrintDefaults()
	}

	opts := Options{}

	flag.StringVar(&opts.dayFrom, "from", "", "from date")
	flag.StringVar(&opts.dayTo, "to", "", "to date")
	flag.Parse()

	var err error
	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "ERROR: missing URL(s)\n")
		flag.Usage()
		os.Exit(1)
	}

	err = doit(&opts, flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

// expand a list of URLs, using the previous URL in the list as the context for the next
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

func doit(opts *Options, urls []string) error {
	urls, err := expandURLs(urls)
	if err != nil {
		return err
	}

	days, err := opts.DayRange()
	if err != nil {
		return err
	}

	client := &http.Client{
		Transport: util.NewPoliteTripper(),
	}

	for _, day := range days {
		timeStamp := day.Format("20060102")
		for _, u := range urls {
			err := doPage(client, u, timeStamp)
			if err != nil {
				return err
			}
		}

	}
	return nil
}

func doPage(client *http.Client, u string, when string) error {
	linkSel := cascadia.MustCompile("a")

	// the "id_" suffix asks for the original html. Without this wayback machine
	// will rewrite all the links to go through itself for easy browsing.
	// This will redirect (302) to the nearest memento to our requested timestamp.
	page := fmt.Sprintf("http://web.archive.org/web/%sid_/%s", when, u)
	root, err := fetchAndParse(client, page)
	if err != nil {
		return fmt.Errorf("%s failed: %s\n", page, err)
	}
	links, err := grabLinks(root, linkSel, u)
	if err != nil {
		return fmt.Errorf("%s error: %s\n", page, err)
	}
	for _, l := range links {
		fmt.Println(l)
	}

	return nil
}

func fetchAndParse(client *http.Client, u string) (*html.Node, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	// NOTE: FT.com always returns 403 if no Accept header is present.
	// Seems like a reasonable thing to send anyway...
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	fmt.Fprintf(os.Stderr, "fetch %s\n", u)

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
