package main

import (
	"fmt"
	"github.com/andybalholm/cascadia"
	"github.com/bcampbell/arts/util"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

type Searcher struct {
	SearchURL string
	Params    url.Values
	// which param holds pagenum (if set, used to iterate through search results)
	PageParam string
	// css selector to find next page
	NextPageSel cascadia.Selector
	// css selector to find article links
	ResultLinkSel cascadia.Selector
	//NoMoreResultsSel cascadia.Selector
	// Number of pages to step through (0=no limit)
	NPages int
}

func (s *Searcher) Run(out io.Writer) error {

	if s.PageParam != "" && s.NextPageSel != nil {
		return fmt.Errorf("PageParam and NextPageSel are mutually exclusive")
	}

	client := &http.Client{Transport: util.NewPoliteTripper()}
	//found := []string{}

	page, err := url.Parse(s.SearchURL)
	if err != nil {
		return err
	}
	s.Params = page.Query()
	// TODO: better merging
	/*
		if len(s.Params) > 0 {
			page.RawQuery = s.Params.Encode()
		}
	*/

	pageCount := 0
	for {
		root, err := fetchAndParse(client, page.String())
		if err != nil {
			return fmt.Errorf("%s failed: %s\n", page.String(), err)
		}

		cnt := 0
		for _, a := range s.ResultLinkSel.MatchAll(root) {
			// embiggen relative URLs
			href := GetAttr(a, "href")
			absURL, err := page.Parse(href)
			if err != nil {
				fmt.Fprintf(os.Stderr, "skip bad url %s\n", href)
				continue
			}
			cnt++
			//			found = append(found, absURL.String())
			fmt.Fprintln(out, absURL.String())
		}

		// finish if "no more results" indicator seen...
		/*
			if s.NoMoreResultsSel != nil {
				if len(s.NoMoreResultsSel(root)) > 0 {
					break
				}
			}
		*/

		// finish if page limit hit...
		pageCount++
		if s.NPages > 0 && pageCount >= s.NPages {
			break
		}

		// finish if no more results on returned page
		if cnt == 0 {
			break
		}

		// determine next page
		if s.NextPageSel != nil {
			// use pagination link to fetch URL of next page
			nexts := s.NextPageSel.MatchAll(root)
			if len(nexts) < 1 {
				return fmt.Errorf("no next-page link found on %s", page.String())
			}

			href := GetAttr(nexts[0], "href")
			nextURL, err := page.Parse(href)
			if err != nil {
				return fmt.Errorf("bad next url %s\n", href)
			}
			page = nextURL
		} else if s.PageParam != "" {

			// build new URL by incrementing pagenum
			pageNum := s.Params.Get(s.PageParam)
			if pageNum == "" {
				pageNum = "1"
			}
			n, err := strconv.Atoi(pageNum)
			if err != nil {
				return fmt.Errorf("Bad page number: '%s'\n", s.Params[s.PageParam])
			}
			s.Params.Set(s.PageParam, strconv.Itoa(n+1))

			page.RawQuery = s.Params.Encode()
		} else {
			// no next-page method - just stop now
			break
		}
	}
	return nil
}
