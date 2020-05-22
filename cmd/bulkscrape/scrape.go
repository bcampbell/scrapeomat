package main

import (
	"fmt"
	"github.com/bcampbell/arts/arts"
	"github.com/bcampbell/arts/util"
	"github.com/bcampbell/scrapeomat/store"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

type ScrapeStats struct {
	Start      time.Time
	End        time.Time
	ErrorCount int
	FetchCount int
	StashCount int
}

func buildHTTPClient() *http.Client {
	// create the http client
	// use politetripper to avoid hammering servers
	transport := util.NewPoliteTripper()
	transport.PerHostDelay = 1 * time.Second
	return &http.Client{
		Transport: transport,
	}
}

func ScrapeArticles(artURLs []string, db store.Store) error {

	client := buildHTTPClient()

	// reset the stats
	stats := ScrapeStats{}
	stats.Start = time.Now()
	defer func() {
		stats.End = time.Now()
		elapsed := stats.End.Sub(stats.Start)
		defer fmt.Printf("finished in %s (%d new articles, %d errors)\n", elapsed, stats.StashCount, stats.ErrorCount)
	}()

	newArts, err := db.WhichAreNew(artURLs)
	if err != nil {
		return err
	}

	for _, artURL := range newArts {
		// grab and stash
		fmt.Printf("%s\n", artURL)
		art, err := scrape(client, artURL)
		if err == nil {
			stats.FetchCount++
			var stashed bool
			stashed, err = stash(art, db)
			if err == nil && stashed {
				stats.StashCount++
			}
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "ERR: %s\n", err)
			stats.ErrorCount++
			// bail out if errors get out of hand
			if stats.ErrorCount > 100+len(newArts)/10 {
				return fmt.Errorf("too many errors (%d)", stats.ErrorCount)
			}
			continue
		}
	}
	return nil
}

func scrape(client *http.Client, artURL string) (*store.Article, error) {
	// FETCH

	//fetchTime := time.Now()
	req, err := http.NewRequest("GET", artURL, nil)
	if err != nil {
		return nil, err
	}
	// NOTE: some sites always returns 403 if no Accept header is present. (ft.com)
	// Seems like a reasonable thing to send anyway...
	//req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept", "*/*")

	// NOTE: Johnson press seems to return 403's if User-Agent is not correct format?
	// In pre-1.15 golang, default was borked.
	// see https://github.com/golang/go/issues/9792
	req.Header.Set("User-Agent", "steno/0.1")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// TODO: could archive to .warc file here

	// EXTRACT
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP error: %s (%s)", resp.Status, artURL)
	}

	rawHTML, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	scraped, err := arts.ExtractFromHTML(rawHTML, artURL)
	if err != nil {
		return nil, err
	}

	art := store.ConvertArticle(scraped)
	return art, nil
}

// stash returns true if the article was added to db.
// Returns false if we already had it.
func stash(art *store.Article, db store.Store) (bool, error) {
	// load into db.
	// check the urls - we might already have it
	ids, err := db.FindURLs(art.URLs)
	if err != nil {
		return false, err
	}
	if len(ids) > 1 {
		return false, fmt.Errorf("resolves to %d articles", len(ids))
	}
	if len(ids) == 1 {
		fmt.Fprintf(os.Stderr, "SKIP (already in db): %s\n", art.CanonicalURL)
		return false, nil
	}
	_, err = db.Stash(art)
	if err != nil {
		return false, err
	}
	return true, nil
}
