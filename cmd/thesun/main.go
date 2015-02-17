package main

import (
	"fmt"
	"github.com/bcampbell/arts/util"
	"os"
	//	"semprini/scrapeomat/paywall"
	"flag"
	"net/http"
	"net/http/cookiejar"
	"semprini/scrapeomat/store"
)

var opts struct {
	verbosity int
}

func main() {

	flag.IntVar(&opts.verbosity, "v", 1, "verbosity of output (0=errors only 1=info 2=debug)")
	var databaseURLFlag = flag.String("db", "", "database connection string (eg postgres://scrapeomat:password@localhost/scrapeomat)")
	flag.Parse()

	// an http client which uses cookies and doesn't hammer the server
	jar, err := cookiejar.New(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}

	client := &http.Client{
		Transport: util.NewPoliteTripper(),
		Jar:       jar,
	}

	// open store
	connStr := *databaseURLFlag
	if connStr == "" {
		connStr = os.Getenv("SCRAPEOMAT_DB")
	}

	if connStr == "" {
		fmt.Fprintf(os.Stderr, "ERROR: no database specified (use -db flag or set $SCRAPEOMAT_DB)\n")
		os.Exit(1)
	}

	db, err := store.NewStore(connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR opening db: %s\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// start....
	scraper := NewSunScraper(opts.verbosity)
	/*
		err = scraper.Login(client)
		if err != nil {
			scraper.errorLog.Printf("login failed: %s\n", err)
			os.Exit(1)
		}
		scraper.infoLog.Printf("logged in ok, I guess.\n")
	*/
	scraper.Start(db, client)
}
