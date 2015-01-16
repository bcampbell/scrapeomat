package main

// hacky little tool to try and grab old articles from a site

import (
	"code.google.com/p/cascadia"
	"fmt"
	"github.com/bcampbell/arts/util"
	"golang.org/x/net/html"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"semprini/scrapeomat/paywall"
)

func main() {
	/*
		var fromDayFlag = flag.String("f", "from", "first day in range (yyyy-mm-dd)")
		var toDayFlag = flag.String("t", "to", "last day in range (yyyy-mm-dd)")
		flag.Parse()
	*/

	//	err := DoTheSun()
	//err := DoFT()
	err := DoTheCourier()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
	}
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

// handles the sun and scottish sun
// nasty and hacky and needs lots of manual intervention.
// set high to the num of articles and manually set up the url for the sun or scottish sun.
// their search links are all ajaxy, so can't just issue a search and
// autoclick the 'next page' link. Instead we iterate through the results
// 10 at a time using the minimal html returned by /search/showMoreAction.do
func DoTheSun() error {
	linkSel := cascadia.MustCompile("li h3 a")

	// need to log in
	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}
	client := &http.Client{
		Transport: util.NewPoliteTripper(),
		Jar:       jar,
	}

	fmt.Fprintf(os.Stderr, "log in\n")
	err = paywall.LoginSun(client)
	if err != nil {
		return err
	}

	//high := 3180
	high := 850
	for offset := 0; offset < high; offset += 10 {

		//u := "http://www.thesun.co.uk/search/showMoreAction.do?pubName=sol&querystring=the&navigators=publication_name:The+Sun&offset=" + fmt.Sprintf("%d", offset) + "&hits=10&sortby=relevance&from=20140828&to=20140917&th=3180"
		u := "http://www.thesun.co.uk/search/showMoreAction.do?pubName=sol&querystring=the&navigators=publication_name:The+Scottish+Sun&offset=" + fmt.Sprintf("%d", offset) + "&hits=10&sortby=date&from=20140828&to=20140917&th=850"

		root, err := fetchAndParse(client, u)
		if err != nil {
			return err
		}
		baseURL, err := url.Parse(u)
		if err != nil {
			return err
		}
		for _, a := range linkSel.MatchAll(root) {
			fmt.Fprintln(os.Stderr, ".")
			href := GetAttr(a, "href")
			absURL, err := baseURL.Parse(href)
			if err != nil {
				fmt.Fprintf(os.Stderr, "skip %s\n", href)
				continue
			}
			fmt.Println(absURL)
		}
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

func DoFT() error {
	linkSel := cascadia.MustCompile(".results .result h3 a")

	// next link doesn't show up here (but does in firefox).
	// Maybe pretending to be a real browser and sending more headers would help?
	//nextPageSel := cascadia.MustCompile(".pagination .next a")

	// so, for now, just iterate page by page until no more results.

	// don't need to log in for search
	client := &http.Client{
		Transport: util.NewPoliteTripper(),
	}

	for page := 1; ; page++ {
		// rpp = results per page
		// fa=facets?
		// s=sort
		u := "http://search.ft.com/search?q=&t=all&rpp=100&fa=people%2Corganisations%2Cregions%2Csections%2Ctopics%2Ccategory%2Cbrand&s=-initialPublishDateTime&f=initialPublishDateTime[2014-08-28T00%3A00%3A00%2C2014-09-18T23%3A59%3A59]&p=" + fmt.Sprintf("%d", page)
		root, err := fetchAndParse(client, u)
		if err != nil {
			return err
		}
		baseURL, err := url.Parse(u)
		if err != nil {
			return err
		}
		cnt := 0
		for _, a := range linkSel.MatchAll(root) {
			href := GetAttr(a, "href")
			absURL, err := baseURL.Parse(href)
			if err != nil {
				fmt.Fprintf(os.Stderr, "skip %s\n", href)
				continue
			}
			cnt++
			fmt.Println(absURL)
		}

		// finish when no more results
		if cnt == 0 {
			break
		}
	}
	return nil
}

func DoTheCourier() error {
	// no specific date range, but you can get the results for the last month/year/week
	linkSel := cascadia.MustCompile(".search-page-results-list .article-title a")
	nextPageSel := cascadia.MustCompile(".search-page-pagination a.next")

	client := &http.Client{Transport: util.NewPoliteTripper()}

	// annoying stopwords in place, so do a bunch of generic-as-possible searches...
	terms := []string{"up", "its", "from", "could", "said", "scotland", "england"}

	for _, term := range terms {
		u := "http://www.thecourier.co.uk/search?q=" + term + "&d=&s=mostRecent&a=&p=pastMonth"
		for {
			// rpp = results per page
			// fa=facets?
			// s=sort

			root, err := fetchAndParse(client, u)
			if err != nil {
				return err
			}
			baseURL, err := url.Parse(u)
			if err != nil {
				return err
			}
			cnt := 0
			for _, a := range linkSel.MatchAll(root) {
				href := GetAttr(a, "href")
				absURL, err := baseURL.Parse(href)
				if err != nil {
					fmt.Fprintf(os.Stderr, "skip %s\n", href)
					continue
				}
				cnt++
				fmt.Println(absURL)
			}

			n := nextPageSel.MatchFirst(root)
			if n == nil {
				fmt.Fprintf(os.Stderr, "fin.\n")
				break
			}

			absNext, err := baseURL.Parse(GetAttr(n, "href"))
			if err != nil {
				return err
			}
			u = absNext.String()
			//	fmt.Fprintf(os.Stderr, "NEXT %s\n", u)
		}
	}
	return nil
}
