package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	"github.com/bcampbell/scrapeomat/store/sqlstore"
)

const usageTxt = `usage: bulkscrape [options] <infile-with-urls>

Scrape articles from a list of urls and load them into a db.
(scrapomat has a similar feature, but requires per-site config).

`

var opts struct {
	db      string
	driver  string
	verbose bool
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usageTxt)
		flag.PrintDefaults()
		os.Exit(2)
	}

	flag.StringVar(&opts.driver, "driver", "", "database driver (defaults to sqlite3 if SCRAPEOMAT_DRIVER is not set)")
	flag.StringVar(&opts.db, "db", "", "database connection string")
	flag.BoolVar(&opts.verbose, "v", false, "verbose")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "ERROR: missing input file.\n")
		os.Exit(1)
	}

	// collect urls
	artURLs := []string{}
	for _, filename := range flag.Args() {
		if opts.verbose {
			fmt.Fprintf(os.Stderr, "reading urls from %s\n", filename)
		}
		var inFile io.Reader
		var err error
		if filename == "-" {
			inFile = os.Stdin
		} else {
			inFile, err = os.Open(filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s\n", filename, err)
				os.Exit(1)
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
			fmt.Fprintf(os.Stderr, "ERROR reading %s: %s\n", filename, err)
			os.Exit(1)
		}
	}
	if opts.verbose {
		fmt.Fprintf(os.Stderr, "got %d urls\n", len(artURLs))
	}

	// set up the database
	db, err := sqlstore.NewWithEnv(opts.driver, opts.db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// scrape them!
	err = ScrapeArticles(artURLs, db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
}
