package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"github.com/bcampbell/scrapeomat/store"
	"github.com/bcampbell/scrapeomat/store/sqlstore"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

var opts struct {
	//verbosity         int
	driver string // database driver
	db     string // db connection string
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
	flag.Parse()

	// Set up store.
	db, err := sqlstore.NewWithEnv(opts.driver, opts.db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR opening db: %s\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Set a crawl running for each site.
	cancelFuncs := []context.CancelFunc{}
	var wg sync.WaitGroup
	for _, siteURL := range flag.Args() {
		// Create a cancellation context for each crawler.
		ctx, cancel := context.WithCancel(context.Background())
		cancelFuncs = append(cancelFuncs, cancel)
		wg.Add(1)
		go func(ctx context.Context, siteURL string, db store.Store) {
			defer wg.Done()
			err := Crawl(ctx, siteURL, db)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s ERR: %s", siteURL, err)
			}
		}(ctx, siteURL, db)
	}

	// Handle Ctrl-C by cancelling all crawls.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		// Wait for signal.
		s := <-sigChan
		fmt.Fprintf(os.Stderr, "Signal received (%s). Stopping scrapers...\n", s)
		for _, cancel := range cancelFuncs {

			fmt.Fprintf(os.Stderr, "CANCEL.\n")
			cancel()
		}
	}()

	fmt.Fprintf(os.Stderr, "waiting...\n")
	wg.Wait()
	fmt.Fprintf(os.Stderr, "exiting.\n")
}
