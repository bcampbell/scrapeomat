package main

import (
	"context"
	"fmt"
	"github.com/bcampbell/scrapeomat/extract"
	"github.com/bcampbell/scrapeomat/store"
	"github.com/gocolly/colly/v2"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type Stats struct {
	Errors        int
	Visited       int
	ArticlesAdded int
	ArticlesHad   int
}

func Crawl(ctx context.Context, siteURL string, db store.Store) error {

	u, err := url.Parse(siteURL)
	if err != nil {
		return err
	}
	hostname := u.Hostname()

	cacheDir := filepath.Join("cache", hostname)

	col := colly.NewCollector(
		colly.AllowedDomains(hostname),
		colly.MaxDepth(3),
		colly.CacheDir(cacheDir),
		colly.StdlibContext(ctx),
	)
	col.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       1 * time.Second,
	})

	col.OnRequest(func(r *colly.Request) {
		err := ctx.Err()
		if err != nil {
			//	fmt.Printf("%s, So skip %s", err, r.URL)
			r.Abort()
		}
	})

	col.OnError(func(_ *colly.Response, err error) {
		if err == context.Canceled {
			fmt.Fprintln(os.Stderr, "%s: CANCELLED", hostname)
		}
	})

	col.OnResponse(func(r *colly.Response) {

		// Decide if we want to scrape this page
		inf, err := extract.Extract(r.Request.URL, r.Headers, r.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: %s\n", err)
			return
		}

		// Remove links for articles which are already in our DB.
		links, err := db.WhichAreNew(inf.Links)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: %s\n", err)
			return
		}

		// Queue all the links for a visit.
		for _, link := range links {
			r.Request.Visit(link)
		}

		fmt.Fprintf(os.Stdout, "%s: %d child links (had %d)\n", r.Request.URL, len(links), len(inf.Links)-len(links))

		// Do we want to scrape _this_ page as an article?
		h := inf.URLHeuristics
		isArticle := (h.Slug || h.Date || h.NumericID)
		if !isArticle {
			fmt.Fprintf(os.Stdout, "  not article\n")
			return
		}
		ids, err := db.FindURLs([]string{inf.URL.String()})
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: %s\n", err)
			return
		}
		if len(ids) == 0 {
			art := toStoreArt(inf)
			artIDs, err := db.Stash(art)
			if err != nil {
				fmt.Fprintf(os.Stderr, "WARN: %s\n", err)
				return
			}
			fmt.Fprintf(os.Stdout, "  stashed id=%d\n", artIDs[0])
		} else {
			fmt.Fprintf(os.Stdout, "  already in db\n")
		}
	})

	//	col.OnHTML("a[href]", func(e *colly.HTMLElement) {
	//		fmt.Println("LINK:", e.Attr("href"))
	//		e.Request.Visit(e.Attr("href"))
	//	})

	col.OnScraped(func(r *colly.Response) {
		//fmt.Println("Finished", r.Request.URL)
	})

	col.Visit(siteURL)
	fmt.Printf("%s Exiting.\n", hostname)
	return nil
}

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
