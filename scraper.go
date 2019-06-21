package main

import (
	"errors"
	"fmt"
	"github.com/bcampbell/arts/arts"
	"github.com/bcampbell/arts/util"
	"github.com/bcampbell/biscuit"
	"github.com/bcampbell/scrapeomat/arc"
	"github.com/bcampbell/scrapeomat/discover"
	"github.com/bcampbell/scrapeomat/paywall"
	"github.com/bcampbell/scrapeomat/store"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"time"
)

type ScrapeStats struct {
	Start      time.Time
	End        time.Time
	ErrorCount int
	FetchCount int

	StashCount int
}

// TODO: factor out a scraper interface, to handle both generic and custom scrapers
// Name() string
// Discover(c *http.Client) ([]string, error)
// DoRun(db *store.Store, c *http.Client) error
// DoRunFromList(arts []string, db *store.Store, c *http.Client) error

type Scraper struct {
	Name       string
	Conf       *ScraperConf
	discoverer *discover.Discoverer
	errorLog   *log.Logger
	infoLog    *log.Logger
	archiveDir string
	stats      ScrapeStats
	runPeriod  time.Duration
	client     *http.Client
	quit       chan struct{}
}

type ScraperConf struct {
	discover.DiscovererDef
	Cookies    bool
	CookieFile string
	PubCode    string
}

var ErrQuit = errors.New("quit requested")

func NewScraper(name string, conf *ScraperConf, verbosity int, archiveDir string) (*Scraper, error) {
	scraper := Scraper{
		Name:       name,
		Conf:       conf,
		archiveDir: archiveDir,
		runPeriod:  3 * time.Hour,
		quit:       make(chan struct{}, 1),
	}

	scraper.errorLog = log.New(os.Stderr, "ERR "+name+": ", 0)
	if verbosity > 0 {
		scraper.infoLog = log.New(os.Stderr, "INF "+name+": ", 0)
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

	// create the http client
	// use politetripper to avoid hammering servers
	var c *http.Client
	if conf.Cookies || (conf.CookieFile != "") {
		jar, err := cookiejar.New(nil)
		if err != nil {
			return nil, err
		}
		// If CookieFile set, load cookies here
		if conf.CookieFile != "" {

			cookieFile, err := os.Open(conf.CookieFile)
			if err != nil {
				return nil, err
			}
			defer cookieFile.Close()
			cookies, err := biscuit.ReadCookies(cookieFile)
			if err != nil {
				return nil, err
			}
			// TODO: use another cookie jar that lets us bulk-load without
			// filtering by URL (SetCookies() kind of assumes you're handling a
			// http response and want to filter dodgy cookies)
			host, err := url.Parse(conf.URL)
			if err != nil {
				return nil, err
			}
			jar.SetCookies(host, cookies)
		}
		c = &http.Client{
			Transport: util.NewPoliteTripper(),
			Jar:       jar,
		}

	} else {
		c = &http.Client{
			Transport: util.NewPoliteTripper(),
		}
	}
	scraper.client = c

	return &scraper, nil
}

func (scraper *Scraper) Login() error {
	login := paywall.GetLogin(scraper.Name)
	if login != nil {
		scraper.infoLog.Printf("Logging in\n")
		err := login(scraper.client)
		if err != nil {
			return fmt.Errorf("Login failed (%s)\n", err)
		}
	}
	return nil
}

func (scraper *Scraper) Discover() ([]string, error) {
	disc := scraper.discoverer

	artLinks, err := disc.Run(scraper.client, scraper.quit)
	if err == discover.ErrQuit {
		return nil, ErrQuit
	}
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
func (scraper *Scraper) Start(db store.Store) {
	for {
		lastRun := time.Now()
		err := scraper.DoRun(db)
		if err == ErrQuit {
			scraper.infoLog.Printf("Quit requested!\n")
			return
		}
		if err != nil {
			scraper.errorLog.Printf("run aborted: %s", err)
		}

		nextRun := lastRun.Add(scraper.runPeriod)
		delay := nextRun.Sub(time.Now())
		scraper.infoLog.Printf("next run at %s (sleeping for %s)\n", nextRun.Format(time.RFC3339), delay)
		// wait for next run, or a quit request
		select {
		case <-scraper.quit:
			scraper.infoLog.Printf("Quit requested!\n")
			return
		case <-time.After(delay):
			scraper.infoLog.Printf("Wakeup!\n")
		}
	}
}

// stop the scraper, at the next opportunity
func (scraper *Scraper) Stop() {
	scraper.quit <- struct{}{}
}

// perform a single scraper run
func (scraper *Scraper) DoRun(db store.Store) error {

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

	err := scraper.Login()
	if err != nil {
		return err
	}

	foundArts, err := scraper.Discover()
	if err != nil {
		return err
	}

	newArts, err := db.WhichAreNew(foundArts)
	if err != nil {
		return fmt.Errorf("WhichAreNew() failed: %s", err)
	}

	stats := scraper.discoverer.Stats
	scraper.infoLog.Printf("found %d articles, %d new (%d pages fetched, %d errors)\n",
		len(foundArts), len(newArts), stats.FetchCount, stats.ErrorCount)

	return scraper.FetchAndStash(newArts, db, false)
}

func uniq(in []string) []string {
	foo := map[string]struct{}{}
	for _, s := range in {
		foo[s] = struct{}{}
	}
	out := make([]string, 0, len(foo))
	for s, _ := range foo {
		out = append(out, s)
	}
	return out
}

// perform a single scraper run, using a list of article URLS instead of invoking the discovery
func (scraper *Scraper) DoRunFromList(arts []string, db store.Store, updateMode bool) error {

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

	// use base url from the discovery config
	baseURL := scraper.discoverer.StartURL

	// process/reject urls using site rules
	cookedArts := []string{}
	rejectCnt := 0
	for _, artURL := range arts {
		cooked, err := scraper.discoverer.CookArticleURL(&baseURL, artURL)
		if err != nil {
			scraper.infoLog.Printf("Reject %s (%s)\n", artURL, err)
			rejectCnt++
			continue
		}
		cookedArts = append(cookedArts, cooked.String())
	}

	// remove any dupes
	cookedArts = uniq(cookedArts)

	var err error
	var newArts []string
	if !updateMode {
		newArts, err = db.WhichAreNew(cookedArts)
		if err != nil {
			return fmt.Errorf("WhichAreNew() failed: %s", err)
		}
	} else {
		// all of `em
		newArts = cookedArts
	}

	scraper.infoLog.Printf("%d articles, %d rejected\n",
		len(newArts), rejectCnt)

	err = scraper.Login()
	if err != nil {
		return err
	}

	return scraper.FetchAndStash(newArts, db, updateMode)
}

func (scraper *Scraper) checkQuit() bool {
	select {
	case <-scraper.quit:
		return true
	default:
		return false
	}
}

func (scraper *Scraper) FetchAndStash(newArts []string, db store.Store, updateMode bool) error {
	//scraper.infoLog.Printf("Start scraping\n")

	// fetch and extract 'em
	for _, artURL := range newArts {
		if scraper.checkQuit() {
			return ErrQuit
		}

		//		scraper.infoLog.Printf("fetch/scrape %s", artURL)
		art, err := scraper.ScrapeArt(artURL)
		if err != nil {
			scraper.errorLog.Printf("%s\n", err)
			scraper.stats.ErrorCount += 1
			if scraper.stats.ErrorCount > 100+len(newArts)/10 {
				return fmt.Errorf("too many errors (%d)", scraper.stats.ErrorCount)
			}
			continue
		}

		// TODO: wrap in transaction...
		// check the urls - we might already have it
		var ids []int
		ids, err = db.FindURLs(art.URLs)
		if err == nil {
			if len(ids) == 1 {
				art.ID = ids[0]
			}
			if len(ids) > 1 {
				err = fmt.Errorf("resolves to %d articles", len(ids))
			}
		}

		if err == nil {
			if art.ID != 0 && !updateMode {
				scraper.errorLog.Printf("already got %s (id %d)\n", artURL, art.ID)
				// TODO: add missing URLs!!!
				continue
			}
			_, err = db.Stash(art)
		}
		if err != nil {
			scraper.errorLog.Printf("stash failure on: %s (on %s)\n", err, artURL)
			scraper.stats.ErrorCount += 1
			if scraper.stats.ErrorCount > 100+len(newArts)/10 {
				return fmt.Errorf("too many errors (%d)", scraper.stats.ErrorCount)
			}
			continue
		}
		scraper.stats.StashCount += 1
		scraper.infoLog.Printf("scraped %s (%d chars)\n", artURL, len(art.Content))
	}
	return nil
}

func (scraper *Scraper) ScrapeArt(artURL string) (*store.Article, error) {
	// FETCH
	fetchTime := time.Now()
	req, err := http.NewRequest("GET", artURL, nil)
	if err != nil {
		return nil, err
	}
	// NOTE: FT.com always returns 403 if no Accept header is present.
	// Seems like a reasonable thing to send anyway...
	//req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept", "*/*")
	if scraper.Conf.UserAgent != "" {
		req.Header.Set("User-Agent", scraper.Conf.UserAgent)
	}

	// other possible headers we might want to fiddle with:
	//req.Header.Set("User-Agent", `Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:28.0) Gecko/20100101 Firefox/28.0`)
	//req.Header.Set("Referrer", "http://...")
	//req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := scraper.client.Do(req)
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
