package main

import (
	//	"bytes"
	"flag"
	"fmt"
	"github.com/bcampbell/scrapeomat/extract"
	//readability "github.com/go-shiori/go-readability"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
)

func main() {

	flag.Parse()

	for _, pageURL := range flag.Args() {
		err := doit(pageURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERR: %s\n", err)
		}
	}
}

func doit(pageURL string) error {
	// **** Fetch ****
	client := &http.Client{}
	req, err := http.NewRequest("GET", pageURL, nil)
	req.Header.Set("Accept", "*/*")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	// **** Read the body ****
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	// **** Extraction ****

	u, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}
	art, err := extract.Extract(u, resp.Header, body)
	if err != nil {
		return nil
	}
	/*
		Title         string
		Byline        string
		Node          *html.Node
		Content       string
		TextContent   string
		Length        int
		Excerpt       string
		SiteName      string
		Image         string
		Favicon       string
		Language      string
		PublishedTime *time.Time
		ModifiedTime  *time.Time
	*/
	dumpArt(pageURL, art)
	return nil
}

func dumpArt(pageURL string, art *extract.ArtInfo) {
	fmt.Printf("URL: %s\n", art.URL)
	fmt.Printf("CanonicalURL: %s\n", art.CanonicalURL)
	fmt.Printf("Title: %s\n", art.Readability.Title)
	fmt.Printf("Byline: %s\n", art.Readability.Byline)
	fmt.Printf("SiteName: %s\n", art.Readability.SiteName)
	fmt.Printf("Published: %v\n", art.Readability.PublishedTime)
	fmt.Printf("Modified: %v\n", art.Readability.ModifiedTime)
	fmt.Printf("URLHeuristics: %v\n", *art.URLHeuristics)
	fmt.Printf("OpenGraph: %v\n", *art.OpenGraph)
	fmt.Printf("------------------------------\n%s\n-------------------------------\n", art.Readability.Content)
}
