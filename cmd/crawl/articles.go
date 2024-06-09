package main

import (
	"context"
	"fmt"
	"github.com/bcampbell/scrapeomat/extract"
	"github.com/bcampbell/scrapeomat/store"
	"io/ioutil"
	"net/http"
	neturl "net/url"
	"os"
	"time"
)

func ScrapeArticles(ctx context.Context, client *http.Client, articleURLs []string, db store.Store) error {

	// Remove links for articles which are already in our DB.
	newArts, err := db.WhichAreNew(articleURLs)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "scraping %d articles (%d were already in db)\n", len(newArts), len(articleURLs)-len(newArts))

	for _, artURL := range newArts {
		req, err := buildRequest(ctx, artURL)
		if err != nil {
			return err
		}

		resp, err := client.Do(req)
		if err != nil {
			return err
		}

		// read in the body
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		// Extract the article data from the page.
		u, err := neturl.Parse(artURL)
		if err != nil {
			return err
		}

		info, err := extract.Extract(u, &resp.Header, body)
		if err != nil {
			return err
		}

		art := toStoreArt(info)
		artIDs, err := db.Stash(art)
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stdout, "Added %s (id=%d)\n", artURL, artIDs[0])
	}
	return nil
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
