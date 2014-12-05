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
	"semprini/scrapeomat/store"
	"strings"
	"sync"
)

func main() {
	var listFlag = flag.Bool("l", false, "List target sites and exit")
	var discoverFlag = flag.Bool("discover", false, "run discovery for target sites, output article links to stdout, then exit")
	var verbosityFlag = flag.Int("v", 1, "verbosity of output (0=errors only 1=info 2=debug)")
	var scrapersConfigFlag = flag.String("s", "scrapers.cfg", "config file for scrapers")
	var archiveDirFlag = flag.String("a", "archive", "archive dir to dump .warc files into")
	var inputListFlag = flag.String("i", "", "input file of URLs (runs scrapers then exit)")
	var databaseURLFlag = flag.String("database", "postgres://scrapeomat:password@localhost/scrapeomat", "database connection string")
	flag.Parse()

	// scraper configuration
	scrapersCfg := struct {
		Scraper map[string]*ScraperConf
	}{}
	err := gcfg.ReadFileInto(&scrapersCfg, *scrapersConfigFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}

	// build scrapers from configuration entries
	scrapers := make(map[string]*Scraper)
	for name, conf := range scrapersCfg.Scraper {
		scraper, err := NewScraper(name, conf, *verbosityFlag, *archiveDirFlag)
		if err != nil {
			panic(err)
		}
		scrapers[name] = scraper
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
	if len(targetSites) == 0 {
		// no sites specified on commandline - do the lot (ie default behaviour)
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

	db, err := store.NewPgStore(*databaseURLFlag)
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
			scraper.DoRunFromList(artURLs, db, client)
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
				scraper.infoLog.Printf("using cookies")
				client = politeClientWithCookies
			} else {
				scraper.infoLog.Printf("not using cookies")
				client = politeClient
			}
			scraper.Start(db, client)
		}()
	}

	wg.Wait()
}
