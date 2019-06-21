package main

//go:generate go-bindata templates static

// run server to provide API and web interface upon a scrapeomat database

import (
	"flag"
	"fmt"
	"github.com/bcampbell/scrapeomat/store"
	"log"
	"os"
)

var opts struct {
	verbosity int
	dbURL     string
	port      int
	prefix    string
	browse    bool
}

func main() {
	//	flag.IntVar(&opts.verbosity, "v", 1, "verbosity of output (0=errors only 1=info 2=debug)")
	flag.StringVar(&opts.dbURL, "db", "", "database connection string (eg postgres://user:password@localhost/scrapeomat) or set $SCRAPEOMAT_DB")
	flag.StringVar(&opts.prefix, "prefix", "", `url prefix (eg "/ukarticles") to allow multiple servers on same port`)
	flag.BoolVar(&opts.browse, "browse", false, `enable html browsing of articles`)
	flag.IntVar(&opts.port, "port", 12345, "port to run server on")
	flag.IntVar(&opts.verbosity, "v", 0, "verbosity (0=errors only, 1=info, 2=debug)")
	flag.Parse()

	errLog := log.New(os.Stderr, "ERR: ", 0)
	var infoLog Logger
	if opts.verbosity > 0 {
		infoLog = log.New(os.Stderr, "INF: ", 0)
	} else {
		infoLog = nullLogger{}
	}

	connStr := opts.dbURL
	if connStr == "" {
		connStr = os.Getenv("SCRAPEOMAT_DB")
	}

	if connStr == "" {
		fmt.Fprintf(os.Stderr, "ERROR: no database specified (use -db flag or set $SCRAPEOMAT_DB)\n")
		os.Exit(1)
	}

	db, err := store.NewSQLStore(connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR opening db: %s\n", err)
		os.Exit(1)
	}
	defer db.Close()

	db.ErrLog = errLog
	if opts.verbosity >= 2 {
		db.DebugLog = log.New(os.Stderr, "store: ", 0)
	}

	// run server
	srv, err := NewServer(db, opts.browse, opts.port, opts.prefix, infoLog, errLog)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}

	err = srv.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
}
