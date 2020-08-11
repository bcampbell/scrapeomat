package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/bcampbell/arts/util" // for politetripper
	htmlesc "html"                   // not to be confused with golang/x/net/html
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// <link rel='https://api.w.org/' href='https://www....com/wp-json/' />
//		BaseAPIURL: "http://www....com/wp-json/",

type Options struct {
	dayFrom, dayTo string
	outputFormat   string // "json", "json-stream"
}

// should be in Options, but want it global for wp.go for now.
var verbose bool

// parseDays converts the date range options into time.Time.
// A missing date is returned as a zero time.
// Ensures the to date is after the from date.
func (opts *Options) parseDays() (time.Time, time.Time, error) {

	const dayFmt = "2006-01-02"
	z := time.Time{}

	from := z
	to := z
	var err error
	if opts.dayFrom != "" {
		from, err = time.Parse(dayFmt, opts.dayFrom)
		if err != nil {
			return z, z, fmt.Errorf("bad 'from' day (%s)", err)
		}
	}

	if opts.dayTo != "" {
		to, err = time.Parse(dayFmt, opts.dayTo)
		if err != nil {
			return z, z, fmt.Errorf("bad 'to' day (%s)", err)
		}

		if !from.IsZero() && to.Before(from) {
			return z, z, fmt.Errorf("bad date range ('from' is after 'to')")
		}
	}

	return from, to, nil
}

func main() {
	flag.Usage = func() {

		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "%s [OPTIONS] <apiURL>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Grab articles from a wordpress site using wp-json API\n")
		fmt.Fprintf(os.Stderr, "<apiURL> is wp REST API root, eg https://www.example.com/wp-json\n")
		fmt.Fprintf(os.Stderr, "Dumps fetched articles as JSON to stdout.\n")
		flag.PrintDefaults()
	}

	opts := Options{}

	flag.StringVar(&opts.dayFrom, "from", "", "from date (YYYY-MM-DD)")
	flag.StringVar(&opts.dayTo, "to", "", "to date (YYYY-MM-DD)")
	flag.StringVar(&opts.outputFormat, "f", "json-stream", "output format: json, json-stream")
	flag.BoolVar(&verbose, "v", false, "verbose")
	flag.Parse()

	var err error
	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "ERROR: missing API URL\n")
		flag.Usage()
		os.Exit(1)
	}

	err = run(flag.Arg(0), &opts)

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

// Our output data format.
// Cut-down version of store.Article to avoid pulling in DB code.
// TODO: pull store.Article into own module!
type Article struct {
	//ID           int    `json:"id,omitempty"`
	CanonicalURL string `json:"canonical_url,omitempty"`
	// all known URLs for article (including canonical)
	// TODO: first url should be considered "preferred" if no canonical?
	//URLs     []string `json:"urls,omitempty"`
	Headline string `json:"headline,omitempty"`
	//Authors  []Author `json:"authors,omitempty"`
	Content string `json:"content,omitempty"`
	// Published contains date of publication.
	// An ISO8601 string is used instead of time.Time, so that
	// less-precise representations can be held (eg YYYY-MM)
	Published string `json:"published,omitempty"`
	Updated   string `json:"updated,omitempty"`
	//Publication Publication `json:"publication,omitempty"`
	Keywords []Keyword `json:"keywords,omitempty"`
	Section  string    `json:"section,omitempty"`
	// space for extra, free-form data
	//	Extra interface{} `json:"extra,omitempty"`
	// Ha! not free-form any more! (bugfix for annoying int/float json issue)
	//Extra *TweetExtra `json:"extra,omitempty"`
}

type Keyword struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

func run(apiURL string, opts *Options) error {
	client := &http.Client{
		Transport: util.NewPoliteTripper(),
	}

	/*	baseURL, err := url.Parse(apiURL)
		if err != nil {
			return err
		}
	*/

	dayFrom, dayTo, err := opts.parseDays()
	if err != nil {
		return err
	}

	out := os.Stdout

	if opts.outputFormat == "json" {
		fmt.Fprintf(out, "[\n")
	}
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	numReceived := 0
	numOutput := 0 // in case some are skipped

	filt := &PostsFilter{
		PerPage: 100,
		Offset:  0,
	}
	if !dayFrom.IsZero() {
		filt.After = dayFrom.Add(-1 * time.Second).Format("2006-01-02T15:04:05")
	}
	if !dayTo.IsZero() {
		filt.Before = dayTo.Add(24 * time.Hour).Format("2006-01-02T15:04:05")
	}

	for {
		expectedTotal, posts, err := FetchPosts(client, apiURL, filt)
		if err != nil {
			return err
		}
		if len(posts) == 0 {
			fmt.Fprintf(os.Stderr, "done.\n")
			break
		}

		for _, p := range posts {
			numReceived++
			art, err := convertPost(client, apiURL, &p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "WARN: Bad post - %s", err)
				continue
			}

			// output it
			if opts.outputFormat == "json" {
				if numOutput > 0 {
					// Fudge our fake js array separator
					fmt.Fprintf(out, ",\n")
				}
			}
			err = enc.Encode(art)
			if err != nil {
				return err

			}
			numOutput++
		}

		fmt.Fprintf(os.Stderr, "received %d/%d\n", numReceived, expectedTotal)
		if numReceived >= expectedTotal {
			break
		}
		// ready for next batch
		filt.Offset += len(posts)
	}
	if opts.outputFormat == "json" {
		// terminate our fake js array
		fmt.Fprintf(out, "\n]\n")
	}
	return nil
}

func parseISO8601(raw string) (time.Time, error) {
	// this isn't ISO8061, but probably close enough for wordpress. We'll see.
	t, err := time.Parse(time.RFC3339, raw)
	return t, err
}

var tagCache = map[int]*Tag{}
var categoryCache = map[int]*Category{}

func convertPost(client *http.Client, apiURL string, p *Post) (*Article, error) {
	baseURL, err := url.Parse(apiURL)
	if err != nil {
		return nil, err
	}

	art := &Article{}
	url, err := baseURL.Parse(p.Link)
	if err != nil {
		return nil, err
	}
	art.CanonicalURL = url.String()

	contentHTML, err := SanitiseHTMLString(p.Content.Rendered)
	if err != nil {
		return nil, err // TODO: should be warning?
	}
	art.Content = contentHTML

	titleText, err := HTMLToText(p.Title.Rendered)
	if err != nil {
		return nil, err // TODO: should be warning?
	}
	art.Headline = SingleLine(htmlesc.UnescapeString(titleText))

	// TODO: should sanitise dates
	art.Published = p.Date
	art.Updated = p.Modified
	// Resolve tags
	for _, tagID := range p.Tags {
		tag, ok := tagCache[tagID]
		if !ok {
			// Need to call the API
			tag, err = FetchTag(client, apiURL, tagID)
			if err != nil {
				return nil, err
			}
			tagCache[tagID] = tag
		}

		kw := Keyword{
			Name: htmlesc.UnescapeString(tag.Name),
			URL:  tag.Link,
		}
		art.Keywords = append(art.Keywords, kw)
	}

	// Resolve categories
	catNames := []string{}
	for _, catID := range p.Categories {
		cat, ok := categoryCache[catID]
		if !ok {
			// Need to call the API
			cat, err = FetchCategory(client, apiURL, catID)
			if err != nil {
				return nil, err
			}
			categoryCache[catID] = cat
		}
		catNames = append(catNames, htmlesc.UnescapeString(cat.Name))
	}
	art.Section += strings.Join(catNames, ", ")
	return art, nil
}
