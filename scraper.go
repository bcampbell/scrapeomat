package main

import (
	"fmt"
	"github.com/bcampbell/arts/arts"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"semprini/scrapeomat/arc"
	"semprini/scrapeomat/discover"
	"semprini/scrapeomat/paywall"
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
	PubCode string
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

func (scraper *Scraper) Login(c *http.Client) error {
	login := paywall.GetLogin(scraper.Name)
	if login != nil {
		scraper.infoLog.Printf("Logging in\n")
		err := login(c)
		if err != nil {
			return fmt.Errorf("Login failed (%s)\n", err)
		}
	}
	return nil
}

func (scraper *Scraper) Discover(c *http.Client) ([]string, error) {
	disc := scraper.discoverer

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
func (scraper *Scraper) Start(db store.Store, c *http.Client) {
	for {
		lastRun := time.Now()
		err := scraper.DoRun(db, c)
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
func (scraper *Scraper) DoRun(db store.Store, c *http.Client) error {

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

	err := scraper.Login(c)
	if err != nil {
		return err
	}

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

	return scraper.FetchAndStash(newArts, db, c)
}

// perform a single scraper run, using a list of article URLS instead of invoking the discovery
func (scraper *Scraper) DoRunFromList(arts []string, db store.Store, c *http.Client) error {

	scraper.infoLog.Printf("start run from list\n")
	// reset the stats
	scraper.stats = ScrapeStats{}
	scraper.stats.Start = time.Now()
	defer func() {
		stats := &scraper.stats
		stats.End = time.Now()
		elapsed := stats.End.Sub(stats.Start)
		defer scraper.infoLog.Printf("finished in %s (%d new articles, %d errors)\n", elapsed, stats.StashCount, stats.ErrorCount)
	}()

	// process/reject urls using site rules
	cookedArts := []string{}
	rejectCnt := 0
	for _, artURL := range arts {
		baseURL, err := url.Parse(artURL)
		if err != nil {
			scraper.errorLog.Printf("%s\n", err)
			rejectCnt++
			continue
		}
		cooked, err := scraper.discoverer.CookArticleURL(baseURL, artURL)
		if err != nil {
			scraper.infoLog.Printf("Reject %s (%s)\n", artURL, err)
			rejectCnt++
			continue
		}
		cookedArts = append(cookedArts, cooked.String())
	}

	newArts, err := db.WhichAreNew(cookedArts)
	if err != nil {
		return fmt.Errorf("WhichAreNew() failed: %s", err)
	}

	scraper.infoLog.Printf("%d new articles, %d rejected\n",
		len(newArts), rejectCnt)

	err = scraper.Login(c)
	if err != nil {
		return err
	}

	return scraper.FetchAndStash(newArts, db, c)
}

func (scraper *Scraper) FetchAndStash(newArts []string, db store.Store, c *http.Client) error {
	//scraper.infoLog.Printf("Start scraping\n")

	// fetch and extract 'em
	for _, artURL := range newArts {
		//		scraper.infoLog.Printf("fetch/scrape %s", artURL)
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
		_, err = db.Stash(art)
		if err != nil {
			scraper.errorLog.Printf("stash failure on: %s (on %s)\n", err, artURL)
			scraper.stats.ErrorCount += 1
			if scraper.stats.ErrorCount > 50+len(newArts)/10 {
				return fmt.Errorf("too many errors (%d)", scraper.stats.ErrorCount)
			}
			continue
		}
		scraper.stats.StashCount += 1
		scraper.infoLog.Printf("scraped %s (%d chars)\n", artURL, len(art.Content))
	}
	return nil
}

func (scraper *Scraper) ScrapeArt(c *http.Client, artURL string) (*store.Article, error) {
	// FETCH
	fetchTime := time.Now()
	req, err := http.NewRequest("GET", artURL, nil)
	if err != nil {
		return nil, err
	}
	// NOTE: FT.com always returns 403 if no Accept header is present.
	// Seems like a reasonable thing to send anyway...
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	// other possible headers we might want to fiddle with:
	//req.Header.Set("User-Agent", `Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:28.0) Gecko/20100101 Firefox/28.0`)
	//req.Header.Set("Referrer", "http://...")
	//req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := c.Do(req)
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

	scraped, err := arts.ExtractFromHTML(rawHTML, artURL)
	if err != nil {
		return nil, err
	}

	art := store.ConvertArticle(scraped)

	if scraper.Conf.PubCode != "" {
		art.Publication.Code = scraper.Conf.PubCode
	} else {
		art.Publication.Code = scraper.Name
	}
	// TODO: set publication code here!
	return art, nil
}
