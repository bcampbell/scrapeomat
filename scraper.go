package main

import (
	"fmt"
	"github.com/bcampbell/arts/arts"
	"github.com/bcampbell/arts/discover"
	"github.com/donovanhide/eventsource"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"semprini/scrapeomat/arc"
	"semprini/scrapeomat/store"
	"time"
)

type ScrapeStats struct {
	Start      time.Time
	End        time.Time
	ErrorCount int
	FetchCount int

	StashCount int
}

type Scraper struct {
	Name       string
	Conf       *ScraperConf
	discoverer *discover.Discoverer
	errorLog   *log.Logger
	infoLog    *log.Logger
	archiveDir string
	stats      ScrapeStats
	runPeriod  time.Duration
}

type ScraperConf struct {
	discover.DiscovererDef
	Cookies bool
}

func NewScraper(name string, conf *ScraperConf, verbosity int, archiveDir string) (*Scraper, error) {
	scraper := Scraper{Name: name, Conf: conf, archiveDir: archiveDir, runPeriod: 6 * time.Hour}

	scraper.errorLog = log.New(os.Stderr, "ERR "+name+": ", log.LstdFlags)
	if verbosity > 0 {
		scraper.infoLog = log.New(os.Stderr, "INF "+name+": ", log.LstdFlags)
	} else {
		scraper.infoLog = log.New(ioutil.Discard, "", 0)
	}

	// set up disoverer
	disc, err := discover.NewDiscoverer(conf.DiscovererDef)
	if err != nil {
		return nil, err
	}
	disc.ErrorLog = scraper.errorLog
	if verbosity > 1 {
		disc.InfoLog = scraper.infoLog
	}
	scraper.discoverer = disc

	return &scraper, nil
}

func (scraper *Scraper) Discover(c *http.Client) ([]string, error) {
	disc := scraper.discoverer

	// if it's a paywalled site, log in first
	if login, got := paywallLogins[scraper.Name]; got {
		scraper.infoLog.Printf("Logging in\n")
		err := login(c)
		if err != nil {
			return nil, fmt.Errorf("Login failed (%s)\n", err)
		}
	}

	artLinks, err := disc.Run(c)
	if err != nil {
		return nil, err
	}

	foundArts := make([]string, 0, len(artLinks))
	for l, _ := range artLinks {
		foundArts = append(foundArts, l.String())
	}
	return foundArts, nil
}

// start the scraper, running it at regular intervals
func (scraper *Scraper) Start(db store.Store, c *http.Client, sseSrv *eventsource.Server) {
	for {
		lastRun := time.Now()
		err := scraper.DoRun(db, c, sseSrv)
		if err != nil {
			scraper.errorLog.Printf("run aborted: %s", err)
		}

		nextRun := lastRun.Add(scraper.runPeriod)
		delay := nextRun.Sub(time.Now())
		scraper.infoLog.Printf("next run at %s (sleeping for %s)\n", nextRun.Format(time.RFC3339), delay)
		time.Sleep(delay)
	}
}

// perform a single scraper run
func (scraper *Scraper) DoRun(db store.Store, c *http.Client, sseSrv *eventsource.Server) error {

	scraper.infoLog.Printf("start run\n")
	// reset the stats
	scraper.stats = ScrapeStats{}
	scraper.stats.Start = time.Now()
	defer func() {
		stats := &scraper.stats
		stats.End = time.Now()
		elapsed := stats.End.Sub(stats.Start)
		defer scraper.infoLog.Printf("run finished in %s (%d new articles, %d errors)\n", elapsed, stats.StashCount, stats.ErrorCount)
	}()

	foundArts, err := scraper.Discover(c)
	if err != nil {
		return fmt.Errorf("discovery failed: %s", err)
	}

	newArts, err := db.WhichAreNew(foundArts)
	if err != nil {
		return fmt.Errorf("WhichAreNew() failed: %s", err)
	}

	stats := scraper.discoverer.Stats
	scraper.infoLog.Printf("found %d articles, %d new (%d pages fetched, %d errors)\n",
		len(foundArts), len(newArts), stats.FetchCount, stats.ErrorCount)

	scraper.infoLog.Printf("Start scraping\n")

	// fetch and extract 'em
	for _, artURL := range newArts {
		art, err := scraper.ScrapeArt(c, artURL)
		if err != nil {
			scraper.errorLog.Printf("%s\n", err)
			scraper.stats.ErrorCount += 1
			if scraper.stats.ErrorCount > 50+len(newArts)/10 {
				return fmt.Errorf("too many errors (%d)", scraper.stats.ErrorCount)
			}
			continue
		}

		// TODO: recheck the urls - we might already have it

		// STASH
		id, err := db.Stash(art)
		if err != nil {
			scraper.errorLog.Printf("stash failure: %s\n", err)
			scraper.stats.ErrorCount += 1
			if scraper.stats.ErrorCount > 50+len(newArts)/10 {
				return fmt.Errorf("too many errors (%d)", scraper.stats.ErrorCount)
			}
			continue
		}
		scraper.stats.StashCount += 1
		scraper.infoLog.Printf("scraped %s %s -> '%s' (%d chars)\n", id, artURL, art.Headline, len(art.Content))

		// TODO: make the store handle notification instead
		if sseSrv != nil {
			// broadcast it to any connected clients
			ev := store.NewArticleEvent(art, id)
			sseSrv.Publish([]string{"all"}, ev)
		}

	}
	return nil
}

func (scraper *Scraper) ScrapeArt(c *http.Client, artURL string) (*arts.Article, error) {
	// FETCH
	fetchTime := time.Now()
	resp, err := c.Get(artURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// ARCHIVE
	err = arc.ArchiveResponse(scraper.archiveDir, resp, artURL, fetchTime)
	if err != nil {
		return nil, err
	}

	// EXTRACT
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP error: %s (%s)", resp.Status, artURL)
	}

	rawHTML, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	art, err := arts.ExtractHTML(rawHTML, artURL)
	if err != nil {
		return nil, err
	}

	return art, nil
}
