package main

/*
TODO:
- specify date range of interest
- skip index files which look like they're outside the date range (often
  they'll have a date in the filename)
- handle gzip files
- read robots.txt to get sitemap files
- add support for googlenews extra fields
- factor out for use as a discovery mechanism in scrapeomat
*/

import (
	//	"encoding/csv"
	"encoding/xml"
	"flag"
	"fmt"
	"github.com/bcampbell/arts/util"
	"net/http"
	"os"
	"strings"
)

type URL struct {
	Loc        string `xml:"loc"`
	ChangeFreq string `xml:"changefreq"`
	Priority   string `xml:"priority"`
	LastMod    string `xml:"lastmod"`
}

type URLSet struct {
	URLs []URL `xml:"url"`
}

type Sitemap struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}
type SitemapIndex struct {
	Sitemaps []Sitemap `xml:"sitemap"`
}

type Result struct {
	XMLName  xml.Name
	Sitemaps []Sitemap `xml:"sitemap"`
	URLs     []URL     `xml:"url"`
	/*URLSet       []URLSet       `xml:"urlset"`
	itemapIndex []SitemapIndex `xml:"sitemapindex"`
	*/
}

func main() {
	flag.Parse()
	for _, sitemapURL := range flag.Args() {
		urls, err := doit(sitemapURL, "")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for _, u := range urls {
			fmt.Println(u.LastMod, u.Loc)
		}
	}
}

func doit(sitemapURL string, indent string) ([]URL, error) {
	result := make([]URL, 0)
	politeClient := &http.Client{
		Transport: util.NewPoliteTripper(),
	}

	resp, err := politeClient.Get(sitemapURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s%s: http error %d", indent, sitemapURL, resp.StatusCode)
	}

	dat := Result{}
	dec := xml.NewDecoder(resp.Body)
	err = dec.Decode(&dat)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	result = append(result, dat.URLs...)
	for _, sitemap := range dat.Sitemaps {

		if !strings.Contains(sitemap.LastMod, "2014") {
			fmt.Fprintf(os.Stderr, "%sSKIP %s (%s)\n", indent, sitemap.Loc, sitemap.LastMod)
			continue
		}
		foo, err := doit(sitemap.Loc, indent+"  ")
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(os.Stderr, "%s%s (%s) => %d urls\n", indent, sitemap.Loc, sitemap.LastMod, len(foo))
		result = append(result, foo...)
	}
	return result, nil
}
