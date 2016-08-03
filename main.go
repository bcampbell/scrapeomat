package main

// the scrapeomat.
// Scrapes configured news sites, shoves the results into a database.
// Also archives the raw html for articles as .warc files for later
// rescraping.

import (
	"bufio"
	"code.google.com/p/gcfg"
	"flag"
	"fmt"
	"github.com/bcampbell/arts/util"
	//	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"semprini/scrapeomat/store"
	"sort"
	"strings"
	"sync"
	"syscall"
)

var opts struct {
	verbosity         int
	scraperConfigPath string
	archivePath       string
	inputFile         string
	updateMode        bool
	discover          bool
	list              bool
	db                string
}

func main() {
	flag.Usage = func() {

		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "%s [OPTIONS] SCRAPER...|ALL\n", os.Args[0])
		fmt.Fprintf(os.Stderr, `
Runs scrapers to find and scrape articles for configured sites.

By default, runs in continuous mode, checking for new articles at regular intervals.

environment vars:

   SCRAPEOMAT_DB - postgres db connection string (same as -db option)

options:
`)
		flag.PrintDefaults()
	}
	flag.IntVar(&opts.verbosity, "v", 1, "verbosity of output (0=errors only 1=info 2=debug)")
	flag.StringVar(&opts.scraperConfigPath, "s", "scrapers", "path for scraper configs")
	flag.StringVar(&opts.archivePath, "a", "archive", "archive dir to dump .warc files into")
	flag.BoolVar(&opts.list, "l", false, "List target sites and exit")
	flag.BoolVar(&opts.discover, "discover", false, "run discovery for target sites, output article links to stdout, then exit")
	flag.StringVar(&opts.inputFile, "i", "", "input file of URLs (runs scrapers then exit)")
	flag.BoolVar(&opts.updateMode, "update", false, "Update articles already in db (when using -i)")
	flag.StringVar(&opts.db, "db", "", "database connection string (eg postgres://scrapeomat:password@localhost/scrapeomat)")
	flag.Parse()

	scrapers, err := buildScrapers()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}

	if opts.updateMode && opts.inputFile == "" {
		fmt.Fprintf(os.Stderr, "ERROR: -update can only be used with -i\n")
		os.Exit(1)
	}

	if opts.list {
		// just list available scrapers and exit
		names := sort.StringSlice{}
		for _, scraper := range scrapers {
			names = append(names, scraper.Name)
		}
		sort.Sort(names)
		for _, name := range names {
			fmt.Println(name)
		}

		return
	}

	// an http client which uses cookies and doesn't hammer the server
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}

	politeClient := &http.Client{
		Transport: util.NewPoliteTripper(),
	}

	politeClientWithCookies := &http.Client{
		Transport: util.NewPoliteTripper(),
		Jar:       jar,
	}

	// which sites?
	targetSites := flag.Args()
	if len(targetSites) == 1 && targetSites[0] == "ALL" {
		// do the lot
		targetSites = []string{}
		for siteName, _ := range scrapers {
			targetSites = append(targetSites, siteName)
		}
	}

	// resolve names to scrapers
	targetScrapers := make([]*Scraper, 0, len(targetSites))
	for _, siteName := range targetSites {
		scraper, got := scrapers[siteName]
		if !got {
			fmt.Fprintf(os.Stderr, "Unknown site '%s'\n", siteName)
			continue
		}
		targetScrapers = append(targetScrapers, scraper)
	}

	if opts.discover {
		// just run discovery phase, print out article URLs, then exit
		for _, scraper := range targetScrapers {
			var client *http.Client
			if scraper.Conf.Cookies {
				client = politeClientWithCookies
			} else {
				client = politeClient
			}
			err := scraper.Login(client)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				continue
			}
			foundArts, _ := scraper.Discover(client)
			for _, a := range foundArts {
				fmt.Println(a)
			}
		}
		return
	}

	connStr := opts.db
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

	// running with input file?
	if opts.inputFile != "" {
		// read in the input URLs from file

		var err error
		artURLs := []string{}

		inFile, err := os.Open(opts.inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR opening input list: %s\n", err)
			os.Exit(1)
		}
		scanner := bufio.NewScanner(inFile)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				artURLs = append(artURLs, line)
			}
		}
		if err = scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR reading %s: %s\n", opts.inputFile, err)
			os.Exit(1)
		}
		if len(targetScrapers) != 1 {
			fmt.Fprintf(os.Stderr, "Only one scraper allowed with -i flag\n")
			// TODO: use scraper host and article patterns to pick a scraper?
			// hardly even need a scraper anyway - the article scraping part is mostly
			// generic...
			// scraper-specific stuff: pubcode, article accept/reject rules, paywall handling... custom stuff (eg json-based articles)
			os.Exit(1)
		}

		// invoke scraper
		for _, scraper := range targetScrapers {
			var client *http.Client
			if scraper.Conf.Cookies {
				scraper.infoLog.Printf("using cookies")
				client = politeClientWithCookies
			} else {
				scraper.infoLog.Printf("not using cookies")
				client = politeClient
			}
			err = scraper.DoRunFromList(artURLs, db, client, opts.updateMode)
			if err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
				os.Exit(1)
			}
		}
		return
	}

	// Run as a server

	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		// wait for signal
		s := <-sigChan
		fmt.Fprintf(os.Stderr, "Signal received (%s). Stopping scrapers...\n", s)
		// stop all the scrapers
		for _, scraper := range targetScrapers {
			scraper.Stop()
		}
	}()

	var wg sync.WaitGroup
	for _, scraper := range targetScrapers {
		wg.Add(1)
		go func(s *Scraper) {
			defer wg.Done()
			var client *http.Client
			if s.Conf.Cookies {
				client = politeClientWithCookies
			} else {
				client = politeClient
			}
			s.Start(db, client)
		}(scraper)
	}

	wg.Wait()
	fmt.Println("Shutdown complete. Exiting.")
}

func buildScrapers() (map[string]*Scraper, error) {
	// scraper configuration
	scrapersCfg := struct {
		Scraper map[string]*ScraperConf
	}{}

	configFiles, err := filepath.Glob(path.Join(opts.scraperConfigPath, "*.cfg"))
	if err != nil {
		return nil, err
	}
	if configFiles == nil {
		return nil, fmt.Errorf("no scraper config files found (in \"%s\")", opts.scraperConfigPath)
	}

	for _, fileName := range configFiles {
		err = gcfg.ReadFileInto(&scrapersCfg, fileName)
		if err != nil {
			return nil, err
		}
	}

	// build scrapers from configuration entries
	scrapers := make(map[string]*Scraper)
	for name, conf := range scrapersCfg.Scraper {
		scraper, err := NewScraper(name, conf, opts.verbosity, opts.archivePath)
		if err != nil {
			return nil, err
		}
		scrapers[name] = scraper
	}
	return scrapers, nil
}
