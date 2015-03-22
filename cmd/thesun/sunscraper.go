package main

// custom scraper for The Sun (www.thesun.co.uk)
//
// The sun website is really a web app. No content is served as HTML.
//
// They still have normal looking URLs, but the HTML returned is just
// a stub which fetches actual content via AJAX. The content is in
// JSON format, which the javascript uses to fill out the page.
// Both articles and sections follow this pattern.
//
// In general, the URL for the JSON data of a page (either section page or article)
// can be derived from the HTML URL by inserting a "web/thesun/" eg
//
// html version:
//   http://www.thesun.co.uk/sol/homepage/showbiz/bizarre/
// json version:
//   http://www.thesun.co.uk/web/thesun/sol/homepage/showbiz/bizarre/
//

import (
	"fmt"
	//	"github.com/bcampbell/arts/util"
	"bytes"
	"encoding/json"
	"github.com/bcampbell/arts/arts/byline"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"semprini/scrapeomat/paywall"
	"semprini/scrapeomat/store"
	"strings"
	"time"
)

type ScrapeStats struct {
	Start      time.Time
	End        time.Time
	ErrorCount int
	FetchCount int

	StashCount int
}

// custom scraper for the sun
type SunScraper struct {
	errorLog  *log.Logger
	infoLog   *log.Logger
	stats     ScrapeStats
	runPeriod time.Duration
}

func NewSunScraper(verbosity int) *SunScraper {
	scraper := &SunScraper{}
	scraper.runPeriod = 3 * time.Hour
	name := "thesun"
	scraper.errorLog = log.New(os.Stderr, "ERR "+name+": ", log.LstdFlags)
	if verbosity > 0 {
		scraper.infoLog = log.New(os.Stderr, "INF "+name+": ", log.LstdFlags)
	} else {
		scraper.infoLog = log.New(ioutil.Discard, "", 0)
	}
	return scraper
}

func (scraper *SunScraper) Name() string {
	return "thesun"
}

func (scraper *SunScraper) Login(client *http.Client) error {
	return paywall.LoginSun(client)
}

type SectionMessage struct {
	ArticleTeasers []struct {
		ArticleType      string `json:"articleType"`
		ArticleUrl       string `json:"articleUrl"`
		ArticleTimestamp int64  `json:"articleTimestamp"`
		SectionUrl       string `json:"sectionUrl"`
	} `json:"articleTeasers"`
}

// Sun has the problem of overwriting old Leaders with new ones at the same url.
// So, for leader URLs, we'll fudge the url by inserting the publication date.
func fudgeLeaderURL(u, dayStr string) string {
	if strings.Contains(u, "/sun_says/") && strings.HasSuffix(u, ".html") {
		u = u[:len(u)-5] + "-" + dayStr + ".html"
	}
	return u
}

var unFudgeLeaderPat = regexp.MustCompile(`(?i)-\d\d\d\d-\d\d-\d\d[.]html`)

// remove any date that might have been inserted in the URL by fudgeLeaderURL()
func unFudgeLeaderURL(u string) string {
	if strings.Contains(u, "/sun_says/") && strings.HasSuffix(u, ".html") {
		u = unFudgeLeaderPat.ReplaceAllString(u, ".html")
	}
	return u
}

func (scraper *SunScraper) Discover(client *http.Client) ([]string, error) {

	baseURL := "http://www.thesun.co.uk"

	queued := []string{}               // sections to visit
	seen := map[string]struct{}{}      // sections visited
	foundArts := map[string]struct{}{} // article links found

	// cheesy hack - The Sun has got the wrong URL for their "sun says" section,
	// so we do a separate check using their search page instead. sigh.
	sunsays, err := scraper.sunsaysDiscover(client)
	if err != nil {
		return nil, err
	}
	for _, foo := range sunsays {
		foundArts[foo] = struct{}{}
	}

	queued = append(queued, baseURL+"/sol/homepage/")
	seen[baseURL+"/sol/homepage/"] = struct{}{}

	for len(queued) > 0 {
		sectionURL := queued[len(queued)-1]
		queued = queued[:len(queued)-1]

		//fmt.Printf("fetching %s\n", sectionURL)
		raw, err := scraper.fetchJSON(client, sectionURL)
		if err != nil {
			// TODO: supply an error threshold?
			return nil, err
		}

		in := bytes.NewBuffer(raw)

		decoder := json.NewDecoder(in)
		var results SectionMessage
		err = decoder.Decode(&results)
		if err != nil {
			return nil, err
		}

		// grab sections
		for _, teaser := range results.ArticleTeasers {
			u := baseURL + teaser.SectionUrl
			if _, got := seen[u]; !got {
				queued = append(queued, u)
				seen[u] = struct{}{}
				//fmt.Printf("new section: %s\n", u)
			}
		}

		// grab articles
		for _, teaser := range results.ArticleTeasers {
			day := time.Unix(teaser.ArticleTimestamp/1000, 0).UTC().Format("2006-01-02")
			u := fudgeLeaderURL(baseURL+teaser.ArticleUrl, day)
			foundArts[u] = struct{}{}
		}
		scraper.infoLog.Printf("%s: has %d articles (now got %d unique articles)\n", sectionURL, len(results.ArticleTeasers), len(foundArts))
	}

	out := make([]string, 0, len(foundArts))
	for artURL, _ := range foundArts {
		out = append(out, artURL)
	}

	return out, nil
}

// partial representation of what we get served up in search results json
// (we only define the fields we're interested in and the json decoder
// ignores the rest)
type SearchResultsMessage struct {
	SearchResults struct {
		Hits []struct {
			ArticleURL       string `json:"articleUrl"`
			ArticleTimestamp int64  `json:"articleTimestamp"`
		} `json:"hits"`
	} `json:"searchResults"`
}

// Fetch the json data which corresponds to an html URL
func (scraper *SunScraper) fetchJSON(client *http.Client, htmlURL string) ([]byte, error) {
	u, err := url.Parse(htmlURL)
	if err != nil {
		return nil, err
	}
	u.Path = "web/thesun/" + u.Path
	jsonURL := u.String()

	scraper.infoLog.Printf("fetching %s\n", jsonURL)

	//fetchTime := time.Now()
	req, err := http.NewRequest("GET", jsonURL, nil)
	if err != nil {
		return nil, err
	}
	// want the json version (HTML version is just a shell which fetches json?)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// ARCHIVE
	/*
		err = arc.ArchiveResponse(scraper.archiveDir, resp, artURL, fetchTime)
		if err != nil {
			return nil, err
		}
	*/

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP error: %s (%s)", resp.Status, jsonURL)
	}

	return ioutil.ReadAll(resp.Body)
}

// cheesy hack - The Sun has got the wrong URL for their "sun says" section,
// so we do a separate check using their search page instead. sigh.
func (scraper *SunScraper) sunsaysDiscover(client *http.Client) ([]string, error) {

	baseURL := "http://www.thesun.co.uk"
	//	searchURL := baseURL + "/web/thesun/search/searchResults.do?querystring=the&filters=date_published_7days:[NOW/DAY-7DAY%20TO%20NOW]&offset=0&hits=1000&sortby=date&order=DESC&bestLinks=off"
	// TODO: step through all results using offset param

	params := url.Values{}
	params.Set("bestLinks", "on")
	params.Set("hits", "20")
	params.Set("querystring", "the sun says")

	searchURL := baseURL + "/web/thesun/search/searchResults.do?" + params.Encode()

	scraper.infoLog.Printf("Looking for \"the say says\" articles using the search page\n")
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {

		return nil, fmt.Errorf("HTTP error %d", resp.StatusCode)
	}

	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	//	fmt.Printf("done:\n\n---------\n%s\n-------\n\n", string(raw))

	in := bytes.NewBuffer(raw)

	decoder := json.NewDecoder(in)
	var results SearchResultsMessage
	err = decoder.Decode(&results)
	if err != nil {
		return nil, err
	}

	found := make([]string, 0, len(results.SearchResults.Hits))
	for _, hit := range results.SearchResults.Hits {
		day := time.Unix(hit.ArticleTimestamp/1000, 0).UTC().Format("2006-01-02")
		artURL := fudgeLeaderURL(baseURL+hit.ArticleURL, day)

		found = append(found, artURL)
	}

	scraper.infoLog.Printf("Found %d 'sun says' articles\n", len(found))
	return found, nil
}

// start the scraper, running it at regular intervals
/*
func (scraper *Scraper) Start(db *store.Store, c *http.Client) {
func (scraper *Scraper) DoRun(db *store.Store, c *http.Client) error {
func (scraper *Scraper) DoRunFromList(arts []string, db *store.Store, c *http.Client) error {
*/

// for decoding json for a single article
type ArtMessage struct {
	ArticleID int64 `json:"articleId"`

	ArticleType                 string `json:"articleType"`
	ArticleSource               string `json:"articleSource"`
	Headline                    string `json:"headline"`
	ArticlePublishedTimestamp   int64  `json:"articlePublishedTimestamp"`
	ArticleLastUpdatedTimestamp int64  `json:"articleLastUpdatedTimestamp"`
	AuthorByline                struct {
		Byline string `json:"byline"`
	} `json:"authorByline"`
	ArticleBody     string `json:"articleBody"`
	SectionSettings struct {
		SectionName string `json:"sectionName"`
	} `json:"sectionSettings"`
	MetaInfo struct {
		CanonicalUrl string `json:"canonicalUrl"`
		Keywords     string `json:"keywords"`
	} `json:"metaInfo"`
}

func (scraper *SunScraper) ScrapeArt(c *http.Client, artURL string) (*store.Article, error) {
	// FETCH

	// note - we grab the unfudged URL, even if we use the fudged version for everything else
	u, err := url.Parse(unFudgeLeaderURL(artURL))
	if err != nil {
		return nil, err
	}
	u.Path = "web/thesun/" + u.Path
	jsonURL := u.String()

	//	fmt.Printf("fetch art %s\n",jsonURL)
	//fetchTime := time.Now()
	req, err := http.NewRequest("GET", jsonURL, nil)
	if err != nil {
		return nil, err
	}
	// want the json version (HTML version is just a shell which fetches json?)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// ARCHIVE
	/*
		err = arc.ArchiveResponse(scraper.archiveDir, resp, artURL, fetchTime)
		if err != nil {
			return nil, err
		}
	*/

	// EXTRACT
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP error: %s (%s)", resp.Status, artURL)
	}

	var parsed ArtMessage
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&parsed)
	if err != nil {
		return nil, err
	}

	//	fmt.Printf("%+v\n", parsed)

	art := &store.Article{}
	art.CanonicalURL = artURL // artURL is fudged, for leaders (otherwise we'd use parsed.MetaInfo.CanonicalUrl)
	art.URLs = []string{art.CanonicalURL}
	art.Headline = parsed.Headline

	authors := byline.Parse(parsed.AuthorByline.Byline)
	art.Authors = []store.Author{}
	for _, au := range authors {
		art.Authors = append(art.Authors, store.Author{Name: au.Name, Email: au.Email})
	}
	art.Content = parsed.ArticleBody
	art.Published = time.Unix(parsed.ArticlePublishedTimestamp/1000, 0).UTC().Format(time.RFC3339)
	art.Updated = time.Unix(parsed.ArticleLastUpdatedTimestamp/1000, 0).UTC().Format(time.RFC3339)

	art.Publication = store.Publication{Code: "thesun", Name: "The Sun", Domain: "www.thesun.co.uk"}
	// TODO: keywords (comma-separated list, needs parsing)
	art.Keywords = []store.Keyword{}
	art.Section = parsed.SectionSettings.SectionName

	//fmt.Printf("%#v\n\n", art)

	return art, nil
}

// start the scraper, running it at regular intervals
func (scraper *SunScraper) Start(db *store.Store, c *http.Client) {
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
func (scraper *SunScraper) DoRun(db *store.Store, c *http.Client) error {

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

	scraper.infoLog.Printf("log in...\n")
	err := scraper.Login(c)
	if err != nil {
		return err
	}
	scraper.infoLog.Printf("logged in ok, I guess.\n")

	foundArts, err := scraper.Discover(c)
	if err != nil {
		return fmt.Errorf("discovery failed: %s", err)
	}

	newArts, err := db.WhichAreNew(foundArts)
	if err != nil {
		return fmt.Errorf("WhichAreNew() failed: %s", err)
	}

	//	stats := scraper.discoverer.Stats
	//	scraper.infoLog.Printf("found %d articles, %d new (%d pages fetched, %d errors)\n",
	//		len(foundArts), len(newArts), stats.FetchCount, stats.ErrorCount)
	scraper.infoLog.Printf("found %d articles, %d new\n", len(foundArts), len(newArts))

	return scraper.FetchAndStash(newArts, db, c)
}

// identical to base scraper...
func (scraper *SunScraper) FetchAndStash(newArts []string, db *store.Store, c *http.Client) error {
	//scraper.infoLog.Printf("Start scraping\n")

	// fetch and extract 'em
	for _, artURL := range newArts {
		//		scraper.infoLog.Printf("fetch/scrape %s", artURL)
		art, err := scraper.ScrapeArt(c, artURL)
		if err != nil {
			scraper.errorLog.Printf("%s\n", err)
			scraper.stats.ErrorCount += 1
			if scraper.stats.ErrorCount > 100+len(newArts)/10 {
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
