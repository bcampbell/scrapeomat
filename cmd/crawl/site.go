package main

import (
	"context"
	"fmt"
	"github.com/bcampbell/scrapeomat/store"
	"log"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
)

type nullLogger struct{}

func (l nullLogger) Printf(format string, v ...interface{}) {
}

type stderrLogger struct{}

func (l stderrLogger) Printf(format string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, format, v...)
}

// Site coordinates discovery and scraping for a single site.
type Site struct {
	URL      string
	ErrLog   store.Logger
	InfoLog  store.Logger
	DebugLog store.Logger
	DB       store.Store
	Client   *http.Client
}

func NewSite(siteURL string, cacheDir string, verbosity int, db store.Store) (*Site, error) {
	url, err := neturl.Parse(siteURL)
	if err != nil {
		return nil, err
	}

	if cacheDir != "" {
		cacheDir = filepath.Join(cacheDir, url.Hostname())
	}

	client, err := buildClient(cacheDir)
	if err != nil {
		return nil, err
	}

	name := url.Hostname()
	name = strings.TrimPrefix(name, "www.")
	site := &Site{
		URL:      siteURL,
		ErrLog:   log.New(os.Stderr, "E "+name+": ", 0),
		InfoLog:  nullLogger{},
		DebugLog: nullLogger{},
		DB:       db,
		Client:   client,
	}

	// Set up logging
	if verbosity >= 1 {
		site.InfoLog = log.New(os.Stderr, "I "+name+": ", 0)
	}
	if verbosity >= 2 {
		site.DebugLog = log.New(os.Stderr, "D "+name+": ", 0)
	}

	return site, nil
}

func (site *Site) Run(ctx context.Context) {
	d := &Discoverer{
		MaxDepth: 2,
		ErrLog:   site.ErrLog,
		InfoLog:  site.InfoLog,
		DebugLog: site.DebugLog,
		visited:  map[string]struct{}{},
	}
	artURLs, err := d.DiscoverArticles(ctx, site.Client, site.URL)
	if err != nil {
		site.ErrLog.Printf("%s: %s\n", site.URL, err)
		return
	}

	scraper := &ArtScraper{
		Client:   site.Client,
		DB:       site.DB,
		ErrLog:   site.ErrLog,
		InfoLog:  site.InfoLog,
		DebugLog: site.DebugLog,
	}

	err = scraper.ScrapeArticles(ctx, artURLs)
	if err != nil {
		site.ErrLog.Printf("%s: %s\n", site.URL, err)
		return
	}
	site.InfoLog.Printf("Done.\n")
}
