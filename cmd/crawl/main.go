package main

import (
	"database/sql"
	"flag"
	"fmt"
	"github.com/bcampbell/scrapeomat/extract"
	"github.com/bcampbell/scrapeomat/store"
	"github.com/bcampbell/scrapeomat/store/sqlstore"
	"github.com/gocolly/colly"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"net/url"
	"os"
	"strings"
	"time"
)

var opts struct {
	//verbosity         int
	driver string // database driver
	db     string // db connection string
}

func main() {
	flag.Usage = func() {

		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "%s [OPTIONS] siteurl\n", os.Args[0])
		fmt.Fprintf(os.Stderr, `
Crawl site and add articles to db.

environment vars:

   SCRAPEOMAT_DRIVER - one of: %s (default driver is sqlite3)
   SCRAPEOMAT_DB - db connection string (same as -db option)

options:
`, strings.Join(sql.Drivers(), ","))
		flag.PrintDefaults()
	}
	flag.StringVar(&opts.driver, "driver", "", "database driver (overrides SCRAPEOMAT_DRIVER)")
	flag.StringVar(&opts.db, "db", "", "database connection string (overrides SCRAPEOMAT_DB)")
	flag.Parse()

	db, err := sqlstore.NewWithEnv(opts.driver, opts.db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR opening db: %s\n", err)
		os.Exit(1)
	}
	defer db.Close()

	for _, siteURL := range flag.Args() {
		err := crawlSite(db, siteURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR during crawl: %s\n", err)
		}
	}

}

func crawlSite(db store.Store, siteURL string) error {

	u, err := url.Parse(siteURL)
	if err != nil {
		return err
	}
	hostname := u.Hostname()

	c := colly.NewCollector(
		colly.AllowedDomains(hostname),
		colly.MaxDepth(2),
		colly.CacheDir("cache"),
	)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       1 * time.Second,
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	c.OnError(func(_ *colly.Response, err error) {
		fmt.Fprintln(os.Stderr, "ERR:", err)
	})

	c.OnResponse(func(r *colly.Response) {
		fmt.Println("Visited", r.Request.URL)

		// Decide if we want to scrape this page
		inf, err := extract.Extract(r.Request.URL, r.Headers, r.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: %s\n", err)
			return
		}

		h := inf.URLHeuristics
		if !(h.Slug || h.Date || h.NumericID) {
			fmt.Printf("  not article\n")
			return
		}

		// Already in db?
		ids, err := db.FindURLs([]string{inf.URL})
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: %s\n", err)
			return
		}
		if len(ids) != 0 {
			return
		}

		fmt.Printf("Stashing %s\n", inf.URL)
		art := toStoreArt(inf)
		_, err = db.Stash(art)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: %s\n", err)
			return
		}

	})

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		//fmt.Println("LINK:", e.Attr("href"))
		e.Request.Visit(e.Attr("href"))
	})

	/*	c.OnHTML("tr td:nth-of-type(1)", func(e *colly.HTMLElement) {
			fmt.Println("First column of a table row:", e.Text)
		})
	*/
	/*
		c.OnXML("//h1", func(e *colly.XMLElement) {
			fmt.Println(e.Text)
		})
	*/

	c.OnScraped(func(r *colly.Response) {
		fmt.Println("Finished", r.Request.URL)
	})

	fmt.Println("Go")
	c.Visit(siteURL)
	return nil
}

func toStoreArt(inf *extract.ArtInfo) *store.Article {
	art := &store.Article{
		CanonicalURL: inf.CanonicalURL,
	}

	if inf.CanonicalURL != "" && inf.CanonicalURL != inf.URL {
		art.URLs = []string{inf.CanonicalURL, inf.URL}
	} else {
		art.URLs = []string{inf.URL}
	}

	//	if inf.Readability != nil {
	art.Headline = inf.Readability.Title
	art.Content = inf.Readability.Content
	if inf.Readability.PublishedTime != nil {
		art.Published = inf.Readability.PublishedTime.Format(time.RFC3339)
	}
	if inf.Readability.ModifiedTime != nil {
		art.Updated = inf.Readability.ModifiedTime.Format(time.RFC3339)
	}
	//	}
	return art
}
