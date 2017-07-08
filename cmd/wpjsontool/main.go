package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/bcampbell/arts/util"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

//		BaseAPIURL: "http://www.proceso.com.mx/wp-json/wp/v2",

type Options struct {
	dayFrom, dayTo string
}

func (opts *Options) parseDays() (time.Time, time.Time, error) {

	const dayFmt = "2006-01-02"
	z := time.Time{}

	from := z
	to := z
	var err error
	if opts.dayFrom != "" {
		from, err = time.Parse(dayFmt, opts.dayFrom)
		if err != nil {
			return z, z, fmt.Errorf("bad 'from' day (%s)", err)
		}
	}

	if opts.dayTo != "" {
		to, err = time.Parse(dayFmt, opts.dayTo)
		if err != nil {
			return z, z, fmt.Errorf("bad 'to' day (%s)", err)
		}

		if !from.IsZero() && to.Before(from) {
			return z, z, fmt.Errorf("bad date range ('from' is after 'to')")
		}
	}

	return from, to, nil
}

func main() {
	flag.Usage = func() {

		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "%s [OPTIONS] <siteURL>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Grab article lists from a wordpress site using wp-json API\n")
		flag.PrintDefaults()
	}

	opts := Options{}

	flag.StringVar(&opts.dayFrom, "from", "", "from date")
	flag.StringVar(&opts.dayTo, "to", "", "to date")
	flag.Parse()

	var err error
	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "ERROR: missing site URL\n")
		flag.Usage()
		os.Exit(1)
	}

	site := flag.Arg(0)
	err = run(site, &opts)

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

type Post struct {
	Link string `json:"link"`
}

func run(siteURL string, opts *Options) error {
	client := &http.Client{
		Transport: util.NewPoliteTripper(),
	}

	base, err := url.Parse(siteURL)
	if err != nil {
		return err
	}

	dayFrom, dayTo, err := opts.parseDays()
	if err != nil {
		return err
	}
	// TODO: use X-WP-Total/X-WP-TotalPages header for pagination!
	page := 1
	for {
		params := url.Values{}
		params.Set("page", strconv.Itoa(page))

		params.Set("per_page", "100")

		if !dayFrom.IsZero() {
			params.Set("after", dayFrom.Add(-1*time.Second).Format("2006-01-02T15:04:05"))
		}
		if !dayTo.IsZero() {
			params.Set("before", dayTo.Add(24*time.Hour).Format("2006-01-02T15:04:05"))
		}

		u := siteURL + "/wp-json/wp/v2/posts?" + params.Encode()

		fmt.Fprintf(os.Stderr, "fetch %s\n", u)

		resp, err := client.Get(u)
		if err != nil {
			return err
		}

		raw, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}
		if resp.StatusCode != 200 {
			return fmt.Errorf("%s: %d\n", resp.StatusCode)
		}

		posts := []Post{}

		err = json.Unmarshal(raw, &posts)
		if err != nil {
			return err
		}
		if len(posts) == 0 {
			fmt.Fprintf(os.Stderr, "done.\n")
			break
		}

		for _, p := range posts {
			full, err := base.Parse(p.Link)
			if err != nil {
				fmt.Fprintf(os.Stderr, "WARN: bad url '%s'", p.Link)
				continue
			}
			fmt.Println(full.String())
		}
		page++
	}
	return nil
}
