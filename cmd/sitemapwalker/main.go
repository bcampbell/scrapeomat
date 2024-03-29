package main

import (
	"compress/gzip"
	"encoding/xml"
	"flag"
	"fmt"
	"github.com/bcampbell/arts/util"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"
)

var opts struct {
	nonrecursive bool
	verbose      bool

	fromDate      string
	toDate        string
	filterSitemap bool
	from          time.Time
	to            time.Time

	maxErrs int
}

type sitemapfile struct {
	SitemapIndex `xml:"sitemapindex"`
	URLset       `xml:"urlset"`
}

type SitemapIndex struct {
	//XMLName xml.Name `xml:"sitemapindex"`
	Sitemap []struct {
		Loc     string `xml:"loc"`
		LastMod string `xml:"lastmod"`
	} `xml:"sitemap"`
}
type URLset struct {
	//XMLName xml.Name `xml:"urlset"`
	URL []struct {
		Loc     string `xml:"loc"`
		LastMod string `xml:"lastmod"`
	} `xml:"url"`
}

func usage() {

	fmt.Fprintf(os.Stderr, `Usage: %s [OPTIONS] [URL] ...
Find pages by scanning sitemap files, starting at the url(s) given.
-to and/or -from can be use to give an (inclusive) range.
<url> lastmod entries are rejected if they are outside that range.
<sitemap> lastmod entries are checked against the range only if -s flag is used.

Options:
`, os.Args[0])

	flag.PrintDefaults()
}

var stats struct {
	fetchCnt      int
	fetchErrs     int
	parseErrs     int // Number of pages which failed to parse as XML
	fetchRejected int
	artsAccepted  int
	artsRejected  int
}

//u := "https://www.thesun.co.uk/sitemap.xml?yyyy=2016&mm=06&dd=20"
func main() {
	// use a politetripper to throttle the request frequency
	client := &http.Client{
		Transport: util.NewPoliteTripper(),
	}

	flag.Usage = usage
	flag.StringVar(&opts.fromDate, "from", "", "ignore links with LastMod before YYYY-MM-DD date")
	flag.StringVar(&opts.toDate, "to", "", "ignore links with LastMod after YYYY-MM-DD date")
	flag.BoolVar(&opts.filterSitemap, "s", false, "apply date filter to <sitemap> lastmod too?")
	flag.BoolVar(&opts.nonrecursive, "n", false, "non-recursive (don't follow <sitemap> links)")
	flag.IntVar(&opts.maxErrs, "e", 10, "maximum errors before bailing out (XML parsing errors don't count)")
	flag.BoolVar(&opts.verbose, "v", false, "verbose")
	flag.Parse()

	var err error
	if opts.fromDate != "" {
		opts.from, err = time.Parse("2006-01-02", opts.fromDate)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: bad 'from' date (%s)\n", err)
			os.Exit(1)
		}
	}
	if opts.toDate != "" {
		opts.to, err = time.Parse("2006-01-02", opts.toDate)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: bad 'to' date (%s)\n", err)
			os.Exit(1)
		}
		opts.to.AddDate(0, 0, 1)
	}

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: no files or urls specified\n")
		os.Exit(1)
	}
	// now run upon each supplied file or url
	for _, u := range flag.Args() {

		err = doit(client, u)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			os.Exit(1)
		}
	}

	if opts.verbose {
		fmt.Fprintf(os.Stderr, "fetched %d files (%d errors, %d skipped, %d badxml), yielded %d links (%d rejected)\n",
			stats.fetchCnt, stats.fetchErrs, stats.fetchRejected, stats.parseErrs, stats.artsAccepted, stats.artsRejected)
	}
}

// try a couple of likely formats for LastMod timestamps
func parseLastMod(lastMod string) (time.Time, error) {
	var t time.Time
	var err error

	fmts := []string{time.RFC3339,
		"2006-01-02T15:04:05Z0700", // eg 2021-04-30T18:10:59Z
		"2006-01-02T15:04Z0700",    // eg 2021-04-30T18:10Z
		"2006-01-02",
	}
	for _, fmt := range fmts {
		t, err = time.Parse(fmt, lastMod)
		if err == nil {
			return t, nil
		}
	}
	return t, err
}

func handleFetchErr(u string, err error) error {
	stats.fetchErrs++
	fmt.Fprintf(os.Stderr, "ERROR fetching %s - %s\n", u, err)
	if stats.fetchErrs < opts.maxErrs {
		return nil // keep going.
	}
	return fmt.Errorf("Too many errors.")
}

// fetch and process a single sitemap xml (file or url)
func doit(client *http.Client, u string) error {
	if opts.verbose {
		fmt.Fprintf(os.Stderr, "fetching %s\n", u)
	}

	foo, err := url.Parse(u)
	if err != nil {
		return handleFetchErr(u, err)
	}

	var in io.ReadCloser
	if foo.Scheme == "" {
		in, err = os.Open(u)
		if err != nil {
			return handleFetchErr(u, err)
		}
	} else {
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			return handleFetchErr(u, err)
		}
		req.Header.Set("Accept", "*/*")
		req.Header.Set("User-Agent", "steno/0.1")

		resp, err := client.Do(req)
		if err != nil {
			return handleFetchErr(u, err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return handleFetchErr(u, fmt.Errorf("http error %d", resp.StatusCode))
		}

		// handle gzipped files
		// (net/http handles compressed Content-Encoding, but this is different.
		// some sites have sitemap.xml.gz files, which are delivered to us
		// verbatim, ie encoded.
		// (Might also be worth checking for .gz extension in URL? Meh. Deal
		// with it if we see a case in the wild not covered by Content-Type).
		if resp.Header.Get("Content-Type") == "application/x-gzip" {
			dec, err := gzip.NewReader(resp.Body)
			if err != nil {
				return fmt.Errorf("gunzip failed: %s", err)
			}
			in = ioutil.NopCloser(dec)
		} else {
			in = resp.Body
		}
	}
	defer in.Close()

	stats.fetchCnt++

	result, err := parse(in)
	if err != nil {
		stats.parseErrs++
		fmt.Fprintf(os.Stderr, "skipping %s - failed to parse (%s)", u, err)
		return nil // keep going!
	}

	// dump out article links
	for _, art := range result.URLset.URL {
		accept := true
		if (!opts.from.IsZero() || !opts.to.IsZero()) && art.LastMod != "" {
			var t time.Time
			t, err = parseLastMod(art.LastMod)
			if err == nil {
				//fmt.Fprintf(os.Stderr, "Parsed '%s' -> %v (from: %v to: %v)\n", art.LastMod, t, opts.from, opts.to)
				if !opts.from.IsZero() && t.Before(opts.from) {
					//fmt.Fprintf(os.Stderr, "Reject '%s' (too early)\n", art.LastMod)
					accept = false // too early
				}
				if !opts.to.IsZero() && (t.Equal(opts.to) || t.After(opts.to)) {
					accept = false // too late
					//fmt.Fprintf(os.Stderr, "Reject '%s' (too late)\n", art.LastMod)
				}
			} else {
				fmt.Fprintf(os.Stderr, "WARN: bad LastMod (%s) in %s (rejecting)\n", art.LastMod, u)
				accept = false
			}

		}

		if accept {
			stats.artsAccepted++
			fmt.Println(art.Loc)
		} else {
			stats.artsRejected++
		}

	}

	// go through any referenced sitemap files
	for _, foo := range result.SitemapIndex.Sitemap {
		if opts.nonrecursive {
			//fmt.Println(foo.Loc)
		} else {
			accept := true
			if opts.filterSitemap && (!opts.from.IsZero() || !opts.to.IsZero()) && foo.LastMod != "" {
				var t time.Time
				t, err = parseLastMod(foo.LastMod)
				if err == nil {
					if !opts.from.IsZero() && t.Before(opts.from) {
						accept = false // too early
					}
					if !opts.to.IsZero() && (t.Equal(opts.to) || t.After(opts.to)) {
						accept = false // too late
					}
				} else {
					fmt.Fprintf(os.Stderr, "WARN: bad LastMod in <sitemap> (%s) in %s (rejecting)\n", foo.LastMod, u)
					accept = false
				}

			}

			if accept {
				err := doit(client, foo.Loc)
				if err != nil {
					return err
				}
			} else {
				if opts.verbose {
					fmt.Fprintf(os.Stderr, "skipping <sitemap> %s (lastmod=%s)\n", foo.Loc, foo.LastMod)
				}
				stats.fetchRejected++
			}
		}
	}
	return nil
}

func parse(in io.Reader) (*sitemapfile, error) {
	dec := xml.NewDecoder(in)
	result := sitemapfile{}

	err := dec.Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("decode failed: %s", err)
	}

	return &result, nil
}
