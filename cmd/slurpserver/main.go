package main

// run server to provide API and web interface upon a scrapeomat database

import (
	"flag"
	"fmt"
	"os"
	"semprini/scrapeomat/server"
	"semprini/scrapeomat/store"
)

var opts struct {
	//	verbosity int
	dbURL  string
	port   int
	prefix string
}

func main() {
	//	flag.IntVar(&opts.verbosity, "v", 1, "verbosity of output (0=errors only 1=info 2=debug)")
	flag.StringVar(&opts.dbURL, "db", "", "database connection string (eg postgres://user:password@localhost/scrapeomat) or set $SCRAPEOMAT_DB")
	flag.StringVar(&opts.prefix, "prefix", "", `url prefix (eg "/ukarticles") to allow multiple servers on same port`)
	flag.IntVar(&opts.port, "port", 12345, "port to run server on")
	flag.Parse()

	connStr := opts.dbURL
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

	// run server
	err = server.Run(db, opts.port, opts.prefix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
}
