package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	"github.com/bcampbell/scrapeomat/grab"
	"github.com/bcampbell/scrapeomat/scrape"
	"github.com/bcampbell/scrapeomat/store"
	"github.com/bcampbell/scrapeomat/store/sqlstore"
)

const usageTxt = `usage: bulkscrape [OPTIONS] [URLFILE]...

Scrape articles from a url lists in URLFILEs and load them into a db.
`

var opts struct {
	db           string
	driver       string
	verbosity    int
	noErrBailout bool
}

type nullLogger struct{}

func (l nullLogger) Printf(format string, v ...interface{}) {
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usageTxt)
		flag.PrintDefaults()
		os.Exit(2)
	}

	flag.StringVar(&opts.driver, "driver", "", "database driver (defaults to sqlite3 if SCRAPEOMAT_DRIVER is not set)")
	flag.StringVar(&opts.db, "db", "", "database connection string")
	flag.IntVar(&opts.verbosity, "v", 0, "0=errors only, 1=info, 2=debug")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "ERROR: missing input file.\n")
		os.Exit(1)
	}

	// Set up logging
	var errLog store.Logger = log.New(os.Stderr, "E: ", 0)
	var infoLog store.Logger = nullLogger{}
	var debugLog store.Logger = nullLogger{}
	if opts.verbosity >= 1 {
		infoLog = log.New(os.Stderr, "I: ", 0)
	}
	if opts.verbosity >= 2 {
		debugLog = log.New(os.Stderr, "D: ", 0)
	}

	// Collect urls
	artURLs := []string{}
	for _, filename := range flag.Args() {
		debugLog.Printf("reading urls from %s\n", filename)
		urls, err := loadURLs(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			os.Exit(1)
		}
		artURLs = append(artURLs, urls...)
	}
	infoLog.Printf("got %d urls\n", len(artURLs))

	// set up the database
	db, err := sqlstore.NewWithEnv(opts.driver, opts.db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// set up the grabber
	grabber, err := grab.NewDefaultGrabber("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}

	// Scrape the articles
	scraper := &scrape.ArtScraper{
		Grabber:  grabber,
		DB:       db,
		ErrLog:   errLog,
		InfoLog:  infoLog,
		DebugLog: debugLog,
	}

	err = scraper.ScrapeArticles(context.Background(), artURLs)
	if err != nil {
		errLog.Printf("%s", err)
		os.Exit(1)
	}
}

// loadURLs() loads a list of urls from a file, one url per line.
// Blank lines are ok and will be ignored.
// If filename is "-", read from stdin.
func loadURLs(filename string) ([]string, error) {
	artURLs := []string{}
	var inFile io.Reader
	var err error
	if filename == "-" {
		inFile = os.Stdin
	} else {
		inFile, err = os.Open(filename)
		if err != nil {
			return nil, fmt.Errorf("%s: %s\n", filename, err)
		}
	}
	scanner := bufio.NewScanner(inFile)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			artURLs = append(artURLs, line)
		}
	}
	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("%s: %s\n", filename, err)
	}
	return artURLs, nil
}
