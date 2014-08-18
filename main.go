package main

import (
	"code.google.com/p/gcfg"
	"flag"
	"fmt"
	"github.com/bcampbell/arts/util"
	"github.com/donovanhide/eventsource"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"semprini/scrapeomat/store"
	"sync"
)

func main() {
	var listFlag = flag.Bool("l", false, "List target sites and exit")
	var discoverFlag = flag.Bool("discover", false, "run discovery for target sites, output article links to stdout, then exit")
	var verbosityFlag = flag.Int("v", 1, "verbosity of output (0=errors only 1=info 2=debug)")
	var noscrapeFlag = flag.Bool("noscrape", false, "disable scrapers")
	var scrapersConfigFlag = flag.String("s", "scrapers.cfg", "config file for scrapers")
	var archiveDirFlag = flag.String("a", "archive", "archive dir to dump .warc files into")
	var databaseURLFlag = flag.String("database", "localhost/scrapeomat", "mongodb database url")
	var portFlag = flag.Int("port", 5678, "port to run SSE server")
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
		// handy to dump out redirects for debugging
		/*
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				fmt.Printf("checkredirect: %s %s\n", req.Method, req.URL)
				if len(via) >= 10 {
					return fmt.Errorf("stopped after 10 redirects")
				}
				return nil
			},
		*/
		Jar: jar,
	}

	// which sites?
	targetSites := flag.Args()
	if len(targetSites) == 0 && *noscrapeFlag == false {
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
			foundArts, _ := scraper.Discover(politeClient)
			for _, a := range foundArts {
				fmt.Println(a)
			}
		}
		return
	}

	db := store.NewMongoStore(*databaseURLFlag)
	defer db.Close()

	// set up sse server
	sseSrv := eventsource.NewServer()
	sseSrv.Register("all", db)
	http.Handle("/all/", sseSrv.Handler("all"))

	//
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", *portFlag))
	if err != nil {
		panic(err)
	}
	defer l.Close()

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
			scraper.Start(db, politeClient, sseSrv)
		}()
	}

	// run the webserver
	http.Serve(l, nil)

	wg.Wait()
}
