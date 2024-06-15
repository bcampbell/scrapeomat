package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/bcampbell/scrapeomat/extract"
	"github.com/bcampbell/scrapeomat/store"
	"io/ioutil"
	"net/http"
	neturl "net/url"
	"time"
)

type ArtScraper struct {
	Client   *http.Client
	DB       store.Store
	ErrLog   store.Logger
	InfoLog  store.Logger
	DebugLog store.Logger
}

func (s *ArtScraper) ScrapeArticles(ctx context.Context, articleURLs []string) error {

	// Remove links for articles which are already in our DB.
	newArts, err := s.DB.WhichAreNew(articleURLs)
	if err != nil {
		return err
	}

	numHad := len(articleURLs) - len(newArts)
	numAdded := 0
	numErrors := 0
	maxErrors := 5 + len(articleURLs)/8

	s.DebugLog.Printf("scraping %d articles (%d were already in db)\n", len(newArts), numHad)

	for _, artURL := range newArts {
		art, err := s.scrapeArt(ctx, artURL)
		if err == nil {
			artIDs, err := s.DB.Stash(art)
			if err == nil {
				numAdded++
				s.DebugLog.Printf("Added %s (id=%d)\n", artURL, artIDs[0])
			}
		}
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return err // Bail out immediately.
			}

			numErrors++
			s.ErrLog.Printf("(%d/%d) %s: %s\n", numErrors, maxErrors, artURL, err)
			if numErrors > maxErrors {
				return fmt.Errorf("Too many errors during article scrape")
			}
		}
	}

	s.InfoLog.Printf("scrape complete. %d added, %d had, %d errors\n", numAdded, numHad, numErrors)

	return nil
}

func (s *ArtScraper) scrapeArt(ctx context.Context, artURL string) (*store.Article, error) {
	req, err := buildRequest(ctx, artURL)
	if err != nil {
		return nil, err
	}

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err // TODO: handle http errors
	}

	// read in the body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Extract the article data from the page.
	u, err := neturl.Parse(artURL)
	if err != nil {
		return nil, err
	}

	info, err := extract.Extract(u, &resp.Header, body)
	if err != nil {
		return nil, err
	}

	art := toStoreArt(info)
	return art, nil
}

// toStoreArt wrangles data from extract.ArtInfo into a form suitable
// to load into the store.
func toStoreArt(inf *extract.ArtInfo) *store.Article {
	art := &store.Article{
		CanonicalURL: inf.CanonicalURL,
	}

	if inf.CanonicalURL != "" && inf.CanonicalURL != inf.URL.String() {
		art.URLs = []string{inf.CanonicalURL, inf.URL.String()}
	} else {
		art.URLs = []string{inf.URL.String()}
	}

	art.Headline = inf.Readability.Title
	art.Content = inf.Readability.Content
	if inf.Readability.PublishedTime != nil {
		art.Published = inf.Readability.PublishedTime.Format(time.RFC3339)
	}
	if inf.Readability.ModifiedTime != nil {
		art.Updated = inf.Readability.ModifiedTime.Format(time.RFC3339)
	}

	art.Publication.Domain = inf.URL.Hostname()
	if inf.OpenGraph != nil {
		og := inf.OpenGraph
		if og.Article != nil {
			// PublishedTime  *time.Time `json:"published_time"`
			// ModifiedTime   *time.Time `json:"modified_time"`
			art.Section = og.Article.Section
			keywords := []store.Keyword{}
			for _, t := range og.Article.Tags {
				kw := store.Keyword{Name: t}
				keywords = append(keywords, kw)
			}
			art.Keywords = keywords

			authors := []store.Author{}
			for _, a := range og.Article.Authors {
				author := store.Author{Name: a}
				authors = append(authors, author)
			}
			art.Authors = authors
		}
	}

	return art
}
