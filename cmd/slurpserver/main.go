package main

//go:generate go-bindata templates static

// run server to provide API and web interface upon a scrapeomat database

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/bcampbell/scrapeomat/store/sqlstore"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

var opts struct {
	verbosity int
	driver    string
	connStr   string
	port      int
	prefix    string
	browse    bool
}

func main() {
	//	flag.IntVar(&opts.verbosity, "v", 1, "verbosity of output (0=errors only 1=info 2=debug)")
	flag.StringVar(&opts.connStr, "db", "", "database connection string (or set SCRAPEOMAT_DB")
	flag.StringVar(&opts.driver, "driver", "", "database driver name (defaults to sqlite3 if SCRAPEOMAT_DRIVER is unset)")
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

	db, err := sqlstore.NewWithEnv(opts.driver, opts.connStr)
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
