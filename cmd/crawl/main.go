package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"github.com/bcampbell/scrapeomat/store/sqlstore"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"io"
	neturl "net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

var opts struct {
	verbosity   int
	driver      string // database driver
	db          string // db connection string
	cacheDir    string // Where to cache http data
	sitesFile   string // csv file to read sitelist from
	loadPubs    bool   // load publications then exit
	grabberKind string // which Grabber to use
}

func main() {
	// Parse options from commandline.
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
	flag.StringVar(&opts.cacheDir, "cache", "", "dir to cache http data \"\"=no cahcing")
	flag.StringVar(&opts.grabberKind, "grabber", "", "how to fetch http (\"\"=built-in default, \"curl\" to use curl)")
	flag.StringVar(&opts.sitesFile, "sites", "", "csv file containing sites to scrape")
	flag.BoolVar(&opts.loadPubs, "loadpubs", false, "Ensure publication entries exist for all given sites, then exit")
	flag.IntVar(&opts.verbosity, "v", 1, "verbosity (0=errors only 1=info 2=debug)")
	flag.Parse()

	// Read in target sites
	siteList := []string{}
	var err error
	if opts.sitesFile != "" {
		siteList, err = readSites(opts.sitesFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR reading %s: %s\n", opts.sitesFile, err)
			os.Exit(1)
		}
	}
	// ...and sites from commandline.
	for _, siteURL := range flag.Args() {
		siteList = append(siteList, siteURL)
	}

	// Set up store.
	db, err := sqlstore.NewWithEnv(opts.driver, opts.db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR opening db: %s\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// just loading publications into db?
	if opts.loadPubs {
		err = loadPubs(db, siteList)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR reading %s: %s\n", opts.sitesFile, err)
			os.Exit(1)
		}
		return
	}

	// Set a crawl running for each site.
	cancelFuncs := []context.CancelFunc{}
	var wg sync.WaitGroup
	for _, siteURL := range siteList {
		grabber, err := initGrabber(siteURL, opts.grabberKind, opts.cacheDir)
		site, err := NewSite(siteURL, grabber, opts.verbosity, db)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s ERR: %s\n", siteURL, err)
			continue
		}
		// Create a cancellation context for each crawler.
		ctx, cancel := context.WithCancel(context.Background())
		cancelFuncs = append(cancelFuncs, cancel)
		wg.Add(1)
		go func(ctx context.Context, s *Site) {
			defer wg.Done()
			site.Run(ctx)
		}(ctx, site)
	}

	// Handle Ctrl-C by cancelling all crawls.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		// Wait for signal.
		s := <-sigChan
		fmt.Fprintf(os.Stderr, "Signal received (%s). Stopping scrapers...\n", s)
		for _, cancel := range cancelFuncs {
			cancel()
		}
	}()

	fmt.Fprintf(os.Stderr, "waiting...\n")
	wg.Wait()
	fmt.Fprintf(os.Stderr, "exiting.\n")
}

func readSites(csvFile string) ([]string, error) {
	out := []string{}

	f, err := os.Open(csvFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comment = '#'

	headers, err := r.Read()
	if err != nil {
		return nil, err
	}

	urlCol := -1
	//	statusCol := -1
	for col, name := range headers {
		switch strings.ToLower(name) {
		case "url":
			urlCol = col
			break
			//		case "status":
			//			statusCol = col
			//			break
		}
	}
	if urlCol == -1 {
		return nil, errors.New("Missing url column")
	}

	for {
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		url := row[urlCol]

		out = append(out, url)
	}
	return out, nil
}

func initGrabber(siteURL string, grabberKind string, cacheDir string) (Grabber, error) {

	switch grabberKind {
	case "":
		{
			url, err := neturl.Parse(siteURL)
			if err != nil {
				return nil, err
			}
			if cacheDir != "" {
				cacheDir = filepath.Join(cacheDir, url.Hostname())
			}

			return NewDefaultGrabber(cacheDir)
		}
	case "curl":
		return NewCurlGrabber()
	default:
		return nil, errors.New("Unknown grabber")
	}
}
