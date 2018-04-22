package main

import (
	"encoding/xml"
	"fmt"
	//"io/ioutil"
	"flag"
	"github.com/bcampbell/arts/util"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

var opts struct {
	nonrecursive bool
	verbose      bool

	fromDate    string
	toDate      string
	filterIndex bool
	from        time.Time
	to          time.Time
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
Filter against page LastMod times using -to and -from.

Options:
`, os.Args[0])

	flag.PrintDefaults()
}

var stats struct {
	fetchCnt      int
	fetchErrs     int
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
	flag.BoolVar(&opts.filterIndex, "f", false, "apply date filter to index file link too?")
	flag.BoolVar(&opts.nonrecursive, "n", false, "non-recursive")
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
		fmt.Fprintf(os.Stderr, "fetched %d files (%d errors, %d skipped), yielded %d links (%d rejected)\n",
			stats.fetchCnt, stats.fetchErrs, stats.fetchRejected, stats.artsAccepted, stats.artsRejected)
	}
}

// try a couple of likely formats for LastMod timestamps
func parseLastMod(lastMod string) (time.Time, error) {
	var t time.Time
	var err error
	t, err = time.Parse(time.RFC3339, lastMod)
	if err == nil {
		return t, nil
	}
	t, err = time.Parse("2006-01-02", lastMod)
	if err == nil {
		return t, nil
	}

	return t, err
}

// fetch and process a single sitemap xml (file or url)
func doit(client *http.Client, u string) error {
	if opts.verbose {
		fmt.Fprintf(os.Stderr, "fetching %s\n", u)
	}

	foo, err := url.Parse(u)
	if err != nil {
		stats.fetchErrs++
		return err
	}

	var in io.ReadCloser
	if foo.Scheme == "" {
		in, err = os.Open(u)
		if err != nil {
			stats.fetchErrs++
			return fmt.Errorf("file open failed: %s", err)
		}
	} else {
		resp, err := client.Get(u)

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			stats.fetchErrs++
			return fmt.Errorf("http error %d", resp.StatusCode)
		}

		if err != nil {
			stats.fetchErrs++
			return fmt.Errorf("fetch failed: %s", err)
		}
		in = resp.Body
	}
	defer in.Close()

	stats.fetchCnt++

	result, err := parse(in)
	if err != nil {
		return err
	}

	// dump out article links
	for _, art := range result.URLset.URL {
		accept := true
		if (!opts.from.IsZero() || !opts.to.IsZero()) && art.LastMod != "" {
			var t time.Time
			t, err = parseLastMod(art.LastMod)
			if err == nil {
				if !opts.from.IsZero() && t.Before(opts.from) {
					accept = false // too early
				}
				if !opts.to.IsZero() && (t.Equal(opts.to) || t.After(opts.to)) {
					accept = false // too late
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
			if (opts.filterIndex && !opts.from.IsZero() || !opts.to.IsZero()) && foo.LastMod != "" {
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
					fmt.Fprintf(os.Stderr, "WARN: bad index LastMod (%s) in %s (rejecting)\n", foo.LastMod, u)
					accept = false
				}

			}

			if accept {
				err := doit(client, foo.Loc)
				if err != nil {
					return err
				}
			} else {
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
