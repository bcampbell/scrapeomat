package main

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
	"path"
	"path/filepath"
	"semprini/scrapeomat/server"
	"semprini/scrapeomat/store"
	"strings"
	"sync"
)

var opts struct {
	verbosity         int
	scraperConfigPath string
	archivePath       string
}

func main() {
	flag.IntVar(&opts.verbosity, "v", 1, "verbosity of output (0=errors only 1=info 2=debug)")
	flag.StringVar(&opts.scraperConfigPath, "s", "scrapers", "path for scraper configs")
	flag.StringVar(&opts.archivePath, "a", "archive", "archive dir to dump .warc files into")
	var listFlag = flag.Bool("l", false, "List target sites and exit")
	var discoverFlag = flag.Bool("discover", false, "run discovery for target sites, output article links to stdout, then exit")
	var inputListFlag = flag.String("i", "", "input file of URLs (runs scrapers then exit)")
	var databaseURLFlag = flag.String("db", "", "database connection string (eg postgres://scrapeomat:password@localhost/scrapeomat)")
	var portFlag = flag.Int("port", -1, "Run api server on port (-1= don't run it)")
	flag.Parse()

	scrapers, err := buildScrapers()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}

	if *listFlag {
		for _, scraper := range scrapers {
			fmt.Println(scraper.Name)
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

	if *discoverFlag {
		for _, siteName := range targetSites {
			scraper, got := scrapers[siteName]
			if !got {
				fmt.Fprintf(os.Stderr, "Unknown site '%s'\n", siteName)
				continue
			}
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

	// running with input file?
	if *inputListFlag != "" {
		// read in the input URLs from file

		var err error
		artURLs := []string{}

		inFile, err := os.Open(*inputListFlag)
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
			fmt.Fprintf(os.Stderr, "ERROR reading %s: %s\n", *inputListFlag, err)
			os.Exit(1)
		}
		if len(targetSites) != 1 {
			fmt.Fprintf(os.Stderr, "Only one scraper allowed with -i flag\n")
			os.Exit(1)
		}

		// invoke scraper
		for _, siteName := range targetSites {
			scraper, got := scrapers[siteName]
			if !got {
				fmt.Fprintf(os.Stderr, "Unknown site '%s'\n", siteName)
				continue
			}
			var client *http.Client
			if scraper.Conf.Cookies {
				scraper.infoLog.Printf("using cookies")
				client = politeClientWithCookies
			} else {
				scraper.infoLog.Printf("not using cookies")
				client = politeClient
			}
			err = scraper.DoRunFromList(artURLs, db, client)
			if err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
				os.Exit(1)
			}
		}
		return
	}

	// Run all the scrapers as goroutines
	var wg sync.WaitGroup
	for _, siteName := range targetSites {
		scraper, got := scrapers[siteName]
		if !got {
			fmt.Fprintf(os.Stderr, "Unknown site '%s'\n", siteName)
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			var client *http.Client
			if scraper.Conf.Cookies {
				client = politeClientWithCookies
			} else {
				client = politeClient
			}
			scraper.Start(db, client)
		}()
	}

	if *portFlag > 0 {

		// run server

		fmt.Printf("start server: http://localhost:%d/all\n", *portFlag)
		err := server.Run(db, *portFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start server on port %d: %s\n", *portFlag, err)
			os.Exit(1)
		}
	}

	wg.Wait()
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
