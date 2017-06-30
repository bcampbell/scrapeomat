package main

// hacky little tool to try and grab old articles from a site

import (
	"flag"
	"fmt"
	"github.com/andybalholm/cascadia"
	"github.com/bcampbell/arts/util"
	"golang.org/x/net/html"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	//	"semprini/scrapeomat/paywall"
	"time"
)

type Options struct {
	dayFrom, dayTo string
	nPages         int
	nStart         int
	//	list           bool // list scrapers then exit
}

func (opts *Options) DayRange() ([]time.Time, error) {
	from, to, err := opts.parseDays()
	if err != nil {
		return nil, err
	}

	// make sure we're at start of day
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)

	out := []time.Time{}
	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		out = append(out, day)
	}
	return out, nil
}

func (opts *Options) parseDays() (time.Time, time.Time, error) {

	const dayFmt = "2006-01-02"
	z := time.Time{}

	var from, to time.Time
	var err error
	if opts.dayFrom == "" {
		return z, z, fmt.Errorf("'from' day required")
	}
	from, err = time.Parse(dayFmt, opts.dayFrom)
	if err != nil {
		return z, z, fmt.Errorf("bad 'from' day (%s)", err)
	}

	if opts.dayTo == "" {
		return z, z, fmt.Errorf("'to' day required")
	}
	to, err = time.Parse(dayFmt, opts.dayTo)
	if err != nil {
		return z, z, fmt.Errorf("bad 'to' day (%s)", err)
	}

	if to.Before(from) {
		return z, z, fmt.Errorf("bad date range ('from' is after 'to')")
	}

	return from, to, nil
}

var scrapers map[string](func(*Options) error) = map[string](func(*Options) error){
	"ft":                DoFT,
	"bbc":               DoBBCNews,
	"thetimes":          DoTheTimes,
	"dailystar":         DoDailyStar,
	"telegraph":         DoTelegraph,
	"croydonadvertiser": DoCroydonAdvertiser,
	"viceuk":            DoViceUK,
	"eluniversal":       DoElUniversal,
	"milenio":           DoMilenio,
	//"thesun": DoTheSun,
}

func main() {
	flag.Usage = func() {

		sites := []string{}
		for site, _ := range scrapers {
			sites = append(sites, site)
		}

		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "%s [OPTIONS] %s\n", os.Args[0], strings.Join(sites, "|"))
		fmt.Fprintf(os.Stderr, "Grab older articles from various sites, dumping the urls out to stdout\n")
		flag.PrintDefaults()
	}

	opts := Options{}

	flag.IntVar(&opts.nPages, "n", 0, "max num of search result pages to fetch")
	flag.IntVar(&opts.nStart, "s", 0, "start value (page, whatever)")
	flag.StringVar(&opts.dayFrom, "from", "", "from date")
	flag.StringVar(&opts.dayTo, "to", "", "to date")
	//flag.BoolVar(&opts.list, "l", false, "list available backfill scrapers, then exit")
	flag.Parse()

	var err error
	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "ERROR: missing publication\n")
		flag.Usage()
		os.Exit(1)
	}

	site := flag.Arg(0)
	scraper := scrapers[site]
	if scraper == nil {
		fmt.Fprintf(os.Stderr, "ERROR: unknown publication '%s'\n", site)
		os.Exit(1)
	}

	err = scraper(&opts)

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

// GetAttr retrieved the value of an attribute on a node.
// Returns empty string if attribute doesn't exist.
func GetAttr(n *html.Node, attr string) string {
	for _, a := range n.Attr {
		if a.Key == attr {
			return a.Val
		}
	}
	return ""
}

// handles the sun and scottish sun
// nasty and hacky and needs lots of manual intervention.
// set high to the num of articles and manually set up the url for the sun or scottish sun.
// their search links are all ajaxy, so can't just issue a search and
// autoclick the 'next page' link. Instead we iterate through the results
// 10 at a time using the minimal html returned by /search/showMoreAction.do
func DoTheSun(opts *Options) error {
	linkSel := cascadia.MustCompile("li h3 a")

	// need to log in
	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}
	client := &http.Client{
		Transport: util.NewPoliteTripper(),
		Jar:       jar,
	}

	//high := 3180
	high := 850
	for offset := 0; offset < high; offset += 10 {

		//u := "http://www.thesun.co.uk/search/showMoreAction.do?pubName=sol&querystring=the&navigators=publication_name:The+Sun&offset=" + fmt.Sprintf("%d", offset) + "&hits=10&sortby=relevance&from=20140828&to=20140917&th=3180"
		u := "http://www.thesun.co.uk/search/showMoreAction.do?pubName=sol&querystring=the&navigators=publication_name:The+Scottish+Sun&offset=" + fmt.Sprintf("%d", offset) + "&hits=10&sortby=date&from=20140828&to=20140917&th=850"

		root, err := fetchAndParse(client, u)
		if err != nil {
			return err
		}
		baseURL, err := url.Parse(u)
		if err != nil {
			return err
		}
		for _, a := range linkSel.MatchAll(root) {
			fmt.Fprintln(os.Stderr, ".")
			href := GetAttr(a, "href")
			absURL, err := baseURL.Parse(href)
			if err != nil {
				fmt.Fprintf(os.Stderr, "skip %s\n", href)
				continue
			}
			fmt.Println(absURL)
		}
	}
	return nil
}


func fetchAndParse(client *http.Client, u string) (*html.Node, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	// NOTE: FT.com always returns 403 if no Accept header is present.
	// Seems like a reasonable thing to send anyway...
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	fmt.Fprintf(os.Stderr, "fetch %s\n", u)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err = fmt.Errorf("HTTP code %d (%s)", resp.StatusCode, u)
		return nil, err
	}

	return html.Parse(resp.Body)
}

func DoFT(opts *Options) error {

	if opts.dayFrom == "" || opts.dayTo == "" {

		return fmt.Errorf("Date range required for FT")
	}

	// FT limits number of pages or results you can iterate through,
	// so perform a separate search for each day
	days, err := genDateRange(opts.dayFrom, opts.dayTo)
	if err != nil {
		return fmt.Errorf("bad date range: %s", err)
	}
	for _, day := range days {

		dayFrom := day.Format(dayFmt)
		dayTo := dayFrom

		searchURL := "http://search.ft.com/search?q=&t=all&rpp=100&fa=people%2Corganisations%2Cregions%2Csections%2Ctopics%2Ccategory%2Cbrand&s=-initialPublishDateTime&f=initialPublishDateTime[" + dayFrom + "T00%3A00%3A00%2C" + dayTo + "T23%3A59%3A59]"
		s := &Searcher{
			SearchURL: searchURL,
			Params:    url.Values{
			/*
				"q":   []string{""},    // querystring
				"rpp": []string{"100"}, // results-per-page
			*/
			},
			PageParam:     "p",
			ResultLinkSel: cascadia.MustCompile(".results .result h3 a"),
			//		NoMoreResultsSel: cascadia.MustCompile(".results .result-list .empty"),
			NPages: 8, // should be enough to cover a day!
		}

		// next link doesn't show up here (but does in firefox).
		// Maybe pretending to be a real browser and sending more headers would help?
		//nextPageSel: cascadia.MustCompile(".pagination .next a")
		// so, for now, just iterate page by page until no more results.

		err := s.Run(os.Stdout)
		if err != nil {
			return err
		}
	}
	return nil
}

func DoTheTimes(opts *Options) error {

	// The Times search doesn't do stopwords, so a search for 'a' does the trick nicely ;-)
	s := &Searcher{
		SearchURL:     "http://www.thetimes.co.uk/search?q=a&sort=date_published&sortorder=desc",
		Params:        url.Values{},
		PageParam:     "p",
		ResultLinkSel: cascadia.MustCompile(".SearchResultList h2.Item-headline a"),
		NPages:        opts.nPages,
	}

	err := s.Run(os.Stdout)
	if err != nil {
		return err
	}
	return nil
}

func DoCroydonAdvertiser(opts *Options) error {

	s := &Searcher{
		SearchURL:     "http://www.croydonadvertiser.co.uk/search/search.html?searchType=&searchPhrase=&where=&orderByOption=dateDesc",
		Params:        url.Values{},
		NextPageSel:   cascadia.MustCompile(`.search-results a[rel="next"]`),
		ResultLinkSel: cascadia.MustCompile(".search-results .channel-list-item a"),
		NPages:        opts.nPages,
	}

	err := s.Run(os.Stdout)
	if err != nil {
		return err
	}
	return nil
}
func DoTheCourier(opts *Options) error {
	// no specific date range, but you can get the results for the last month/year/week
	linkSel := cascadia.MustCompile(".search-page-results-list .article-title a")
	nextPageSel := cascadia.MustCompile(".search-page-pagination a.next")

	client := &http.Client{Transport: util.NewPoliteTripper()}

	// annoying stopwords in place, so do a bunch of generic-as-possible searches...
	terms := []string{"up", "its", "from", "could", "said", "scotland", "england"}

	for _, term := range terms {
		u := "http://www.thecourier.co.uk/search?q=" + term + "&d=&s=mostRecent&a=&p=pastMonth"
		for {
			// rpp = results per page
			// fa=facets?
			// s=sort

			root, err := fetchAndParse(client, u)
			if err != nil {
				return err
			}
			baseURL, err := url.Parse(u)
			if err != nil {
				return err
			}
			cnt := 0
			for _, a := range linkSel.MatchAll(root) {
				href := GetAttr(a, "href")
				absURL, err := baseURL.Parse(href)
				if err != nil {
					fmt.Fprintf(os.Stderr, "skip %s\n", href)
					continue
				}
				cnt++
				fmt.Println(absURL)
			}

			n := nextPageSel.MatchFirst(root)
			if n == nil {
				fmt.Fprintf(os.Stderr, "fin.\n")
				break
			}

			absNext, err := baseURL.Parse(GetAttr(n, "href"))
			if err != nil {
				return err
			}
			u = absNext.String()
			//	fmt.Fprintf(os.Stderr, "NEXT %s\n", u)
		}
	}
	return nil
}

const dayFmt = "2006-01-02"

// TODO: kill this
func genDateRange(dayFrom, dayTo string) ([]time.Time, error) {

	var from, to time.Time
	if dayFrom == "" {
		return nil, fmt.Errorf("from day required")
	}
	from, err := time.Parse(dayFmt, dayFrom)
	if err != nil {
		return nil, err
	}

	if dayTo == "" {
		now := time.Now()
		to = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	} else {
		to, err = time.Parse(dayFmt, dayTo)
		if err != nil {
			return nil, err
		}
	}

	if to.Before(from) {
		return nil, fmt.Errorf("to day is before from")
	}

	out := []time.Time{}
	end := to.AddDate(0, 0, 1)
	for day := from; day.Before(end); day = day.AddDate(0, 0, 1) {
		out = append(out, day)
	}
	return out, nil
}

// daily star has handy archive pages, one per day:
// http://www.dailystar.co.uk/sitearchive/YYYY/M/D
func DoDailyStar(opts *Options) error {
	days, err := genDateRange(opts.dayFrom, opts.dayTo)
	if err != nil {
		return err
	}
	client := &http.Client{
		Transport: util.NewPoliteTripper(),
	}
	linkSel := cascadia.MustCompile(".sitemap li a")
	for _, day := range days {
		page := fmt.Sprintf("http://www.dailystar.co.uk/sitearchive/%d/%d/%d",
			day.Year(), day.Month(), day.Day())
		root, err := fetchAndParse(client, page)
		if err != nil {
			return fmt.Errorf("%s failed: %s\n", page, err)
		}
		links, err := grabLinks(root, linkSel, page)
		if err != nil {
			return fmt.Errorf("%s error: %s\n", page, err)
		}
		for _, l := range links {
			fmt.Println(l)
		}
	}

	return nil
}

func grabLinks(root *html.Node, linkSel cascadia.Selector, baseURL string) ([]string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	out := []string{}
	for _, a := range linkSel.MatchAll(root) {
		link, err := href(a, u)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s BAD link: '%s'\n", baseURL, err)
			continue
		}
		out = append(out, link)
	}
	return out, nil
}

func href(anchor *html.Node, baseURL *url.URL) (string, error) {
	h := GetAttr(anchor, "href")
	absURL, err := baseURL.Parse(h)
	if err != nil {
		return "", fmt.Errorf("bad href (%s): %s", h, err)
	}
	return absURL.String(), nil
}

func DoBBCNews(opts *Options) error {
	// BBC has a search facility, but doesn't seem to have an option to
	// sort by date... so it's a matter of stepping through huge numbers of results
	// in the hope that we pick up what we need. Sigh.

	// TODO: could discard articles outside desired date range... but probably not
	// worth the effort

	/*
		dFrom, err := time.Parse("2006-01-02", dayFrom)
		if err != nil {
			return err
		}
		dTo, err := time.Parse("2006-01-02", dayTo)
		if err != nil {
			return err
		}
		dTo.AddDate(0, 0, 1)
		discardCnt := 0
	*/
	artSel := cascadia.MustCompile(`article`)
	linkSel := cascadia.MustCompile(`h1 a`)
	dateSel := cascadia.MustCompile(`time`)

	const MAXPAGE = 1000
	for pageNum := 1; pageNum <= MAXPAGE; pageNum++ {
		// http://www.bbc.co.uk/search?q=the&sa_f=search-serp&filter=news
		// http://www.bbc.co.uk/search/more?page=2&q=the&sa_f=search-serp&filter=news
		page := `http://www.bbc.co.uk/search/more?page=2&q=the&sa_f=search-serp&filter=news`

		baseURL, err := url.Parse(page)
		if err != nil {
			return err
		}

		client := &http.Client{Transport: util.NewPoliteTripper()}
		root, err := fetchAndParse(client, page)
		if err != nil {
			return fmt.Errorf("%s failed: %s\n", page, err)
		}

		for _, art := range artSel.MatchAll(root) {

			d := dateSel.MatchFirst(art)
			a := linkSel.MatchFirst(art)

			if d == nil {
				return fmt.Errorf("%s: missing date\n", page)
			}
			if a == nil {
				return fmt.Errorf("%s: missing link\n", page)
			}

			artURL, err := href(a, baseURL)
			if err != nil {
				return fmt.Errorf("%s error: %s\n", page, err)
			}

			/*
				// TODO: date range filtering here...
				dt, err := time.Parse(time.RFC3339, GetAttr(d, "datetime"))
				if err != nil {
					return err
				}

				//if (dt.Equal(dFrom)||dt.After(dFrom)) && dt.Before(dTo) {...}
			*/

			fmt.Println(artURL)
		}
		//html.Render(os.Stdout, root)
		//fmt.Printf("\n")

	}
	return nil
}
