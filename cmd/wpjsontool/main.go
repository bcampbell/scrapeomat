package main

import (
	"fmt"
	"github.com/bcampbell/arts/util"
	"net/http"
	//"net/url"
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
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
		fmt.Fprintf(os.Stderr, "%s [OPTIONS] %s\n", os.Args[0])
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

	for {
		u := siteURL + "/wp-json/wp/v2/posts"

		resp, err := client.Get(u)
		if err != nil {
			return err
		}

		raw, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}
		posts := []Post{}

		err = json.Unmarshal(raw, &posts)
		if err != nil {
			return err
		}
		for _, p := range posts {
			fmt.Println(p.Link)
		}
		return nil
	}
}
