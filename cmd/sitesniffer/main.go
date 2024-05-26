package main

import (
	"encoding/csv"
	//"errors"
	"flag"
	"fmt"
	"github.com/andybalholm/cascadia"
	"github.com/bcampbell/arts/util"
	"golang.org/x/net/html"
	"net/http"
	"os"
	"reflect"
	"time"
	//"github.com/asaskevich/govalidator"
	//"github.com/dyatlov/go-htmlinfo/htmlinfo"
	//	"github.com/namsral/microdata"
	//	"willnorris.com/go/microformats"
)

var opts struct {
	csvInFile   string
	dumpHeaders bool
}

func main() {
	flag.StringVar(&opts.csvInFile, "c", "", "csv file with list of urls")
	flag.BoolVar(&opts.dumpHeaders, "d", false, "dump response headers")
	flag.Parse()

	transport := util.NewPoliteTripper()
	transport.PerHostDelay = 1 * time.Second
	client := &http.Client{
		Transport: transport,
	}

	if opts.csvInFile != "" {
		err := processCSVList(client, opts.csvInFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERR: %s\n", err)
			os.Exit(1)
		}
	}

	if len(flag.Args()) > 0 {
		w := csv.NewWriter(os.Stdout)
		for _, siteURL := range flag.Args() {
			info := sniffSite(client, siteURL)
			err := w.Write(info.toRow())
			if err != nil {
				fmt.Fprintf(os.Stderr, "ERR: %s\n", err)
				os.Exit(1)
			}
			w.Flush()
		}
	}
}

type Info struct {
	URL          string
	Status       string // numeric HTTP code or error message
	CanonicalURL string // "" if none
	NumLDJSON    int
	Generator    string // the CMS
	// http status
	// cms (wordpress etc)
	// canonical url
}

func (info *Info) toRow() []string {
	canonical := info.CanonicalURL
	if info.CanonicalURL == info.URL {
		canonical = ""
	}
	return []string{info.URL, info.Status, canonical, fmt.Sprintf("%d", info.NumLDJSON), info.Generator, ""}
}

func processCSVList(client *http.Client, csvFilename string) error {
	f, err := os.Open(csvFilename)
	if err != nil {
		return err
	}

	data, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return err
	}

	w := csv.NewWriter(os.Stdout)

	for _, row := range data[1:] {
		siteURL := row[0]
		//fmt.Printf("%s...", siteURL)
		info := sniffSite(client, siteURL)
		//fmt.Printf("got.\n")
		err = w.Write(info.toRow())
		if err != nil {
			return err
		}
		w.Flush()
	}
	return nil
}

func sniffSite(client *http.Client, siteURL string) *Info {
	info := &Info{URL: siteURL}

	// Construct request
	req, err := http.NewRequest("GET", siteURL, nil)
	if err != nil {
		t := reflect.TypeOf(err)
		info.Status = t.String()
		return info
	}
	// NOTE: FT.com always returns 403 if no Accept header is present.
	// Seems like a reasonable thing to send anyway...
	//req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept", "*/*")
	//	req.Header.Set("User-Agent", scraper.Conf.UserAgent)

	// other possible headers we might want to fiddle with:
	//req.Header.Set("User-Agent", `Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:28.0) Gecko/20100101 Firefox/28.0`)
	//req.Header.Set("Referrer", "http://...")
	//req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := client.Do(req)
	if err != nil {
		//if errors.Is(err, net.ErrClosed) {
		//}
		t := reflect.TypeOf(err)
		info.Status = t.String()
		return info
	}

	if opts.dumpHeaders {
		resp.Header.Write(os.Stdout)
	}

	info.Status = fmt.Sprintf("%d", resp.StatusCode)

	// Parse the html
	tree, err := html.Parse(resp.Body)
	if err != nil {
		t := reflect.TypeOf(err)
		info.Status = t.String()

		return info
	}

	canonicalURL, err := checkRelCanonical(tree)
	if err != nil {
		t := reflect.TypeOf(err)
		info.Status = t.String()
		return info
	}
	info.CanonicalURL = canonicalURL
	info.NumLDJSON = len(ldJSONsel.MatchAll(tree))
	info.Generator = checkGenerator(tree)
	return info
}

// GetAttr retrieves the value of an attribute on a node.
// Returns empty string if attribute doesn't exist.
func GetAttr(n *html.Node, attr string) string {
	for _, a := range n.Attr {
		if a.Key == attr {
			return a.Val
		}
	}
	return ""
}

var ldJSONsel cascadia.Selector = cascadia.MustCompile(`script[type="application/ld+json"]`)
var relCanonicalSel cascadia.Selector = cascadia.MustCompile(`head link[rel="canonical"]`)

func checkRelCanonical(tree *html.Node) (string, error) {
	for _, rel := range relCanonicalSel.MatchAll(tree) {

		canonicalURL := GetAttr(rel, "href")
		if len(canonicalURL) > 0 {
			return canonicalURL, nil
		}
	}
	return "", nil
}

var metaGeneratorSel cascadia.Selector = cascadia.MustCompile(`meta[name="generator"]`)

func checkGenerator(tree *html.Node) string {
	gen := metaGeneratorSel.MatchFirst(tree)
	if gen == nil {
		return ""
	}
	return GetAttr(gen, "content")
}
