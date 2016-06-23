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
)

var opts struct {
	nonrecursive bool
	verbose      bool
}

type sitemapfile struct {
	SitemapIndex `xml:"sitemapindex"`
	URLset       `xml:"urlset"`
}

type SitemapIndex struct {
	//XMLName xml.Name `xml:"sitemapindex"`
	Sitemap []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}
type URLset struct {
	//XMLName xml.Name `xml:"urlset"`
	URL []struct {
		Loc string `xml:"loc"`
	} `xml:"url"`
}

//u := "https://www.thesun.co.uk/sitemap.xml?yyyy=2016&mm=06&dd=20"
func main() {
	// use a politetripper to throttle the request frequency
	client := &http.Client{
		Transport: util.NewPoliteTripper(),
	}

	flag.BoolVar(&opts.nonrecursive, "n", false, "non-recursive")
	flag.BoolVar(&opts.verbose, "v", false, "verbose")
	flag.Parse()

	for _, u := range flag.Args() {

		err := doit(client, u)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			os.Exit(1)
		}
	}
}

func doit(client *http.Client, u string) error {
	if opts.verbose {
		fmt.Fprintf(os.Stderr, "fetching %s\n", u)
	}

	foo, err := url.Parse(u)
	if err != nil {
		return err
	}

	var in io.ReadCloser
	if foo.Scheme == "" {
		in, err = os.Open(u)
		if err != nil {
			return fmt.Errorf("file open failed: %s", err)
		}
	} else {
		resp, err := client.Get(u)

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("http error %d", resp.StatusCode)
		}

		if err != nil {
			return fmt.Errorf("fetch failed: %s", err)
		}
		in = resp.Body
	}
	defer in.Close()

	result, err := parse(in)
	if err != nil {
		return err
	}

	// dump out article links
	for _, art := range result.URLset.URL {
		fmt.Println(art.Loc)
	}

	// go through any referenced sitemap files
	for _, foo := range result.SitemapIndex.Sitemap {
		if opts.nonrecursive {
			fmt.Println(foo.Loc)
		} else {
			err := doit(client, foo.Loc)
			if err != nil {
				return err
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
