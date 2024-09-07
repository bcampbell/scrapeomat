package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"github.com/bcampbell/scrapeomat/slurp"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	neturl "net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var opts struct {
	server       string
	from, to     string
	pubs         stringArgs
	csvSiteFiles stringArgs
	termWidth    int
	csv          bool
}

type stringArgs []string

func (p *stringArgs) String() string         { return fmt.Sprintf("%s", *p) }
func (p *stringArgs) Set(value string) error { *p = append(*p, value); return nil }

func init() {
	flag.StringVar(&opts.from, "from", "", "from date")
	flag.StringVar(&opts.to, "to", "", "to date")
	flag.IntVar(&opts.termWidth, "w", 0, "output width (0=auto)")
	flag.StringVar(&opts.server, "s", "http://localhost:12345", "`url` of API server to query")
	flag.BoolVar(&opts.csv, "c", false, "output as csv rather than ascii-art")
	flag.Var(&opts.pubs, "p", "publication code(s) to query")
	flag.Var(&opts.csvSiteFiles, "sites", "master site list csv files with a 'url' column (ensures those sites will appear in results even if no articles)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, `
Queries a slurp server and displays summary of article counts
using a noddy ascii art chart.
`)
		flag.PrintDefaults()
	}
}

func main() {

	flag.Parse()

	if !opts.csv && opts.termWidth == 0 {
		var err error
		opts.termWidth, err = detectTermWidth()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR detecting terminal width: %s\n", err)
			os.Exit(2)
		}
	}

	filt := slurp.Filter{}

	if opts.from != "" {
		from, err := time.Parse("2006-01-02", opts.from)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Bad from: %s\n", err)
			os.Exit(2)
		}
		filt.PubFrom = from
	}

	if opts.to != "" {
		to, err := time.Parse("2006-01-02", opts.to)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Bad to: %s\n", err)
			os.Exit(2)
		}
		filt.PubTo = to
	}

	filt.PubCodes = opts.pubs

	// Load in any site lists.
	masterSiteList := []string{}
	for _, listFile := range opts.csvSiteFiles {
		urls, err := readSites(listFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %s\n", listFile, err)
			os.Exit(2)
		}
		masterSiteList = append(masterSiteList, urls...)
	}

	// Call the API to fetch the raw summary data.
	slurper := slurp.NewSlurper(opts.server)
	raw, err := slurper.Summary(&filt)

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(2)
	}

	// If we have a master site list, add in empty placeholder entries for any
	// which don't show up in the results, so the user can spot any gaps in coverage.
	addMissingSites(raw, masterSiteList)

	// Cook the raw data to order by day and fill in missing days.
	cooked := slurp.CookSummary(raw)

	// Output the results!
	if opts.csv {
		err = dumpCSV(cooked)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			os.Exit(1)
		}
	} else {
		dump(cooked, opts.termWidth)
	}
}

func detectTermWidth() (int, error) {
	fd := int(os.Stdout.Fd())
	if !terminal.IsTerminal(fd) {
		return 0, fmt.Errorf("Not a terminal")
	}
	w, _, err := terminal.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 0, err
	}
	return w, nil
}

func weekday(day string) string {
	t, _ := time.Parse("2006-01-02", day)
	return t.Weekday().String()[:1]
}

func dump(cooked *slurp.CookedSummary, termW int) {

	numReserve := len(fmt.Sprintf("%d", cooked.Max))

	w := termW - (1 + 1 + 10 + 1 + numReserve + 1 + 1)

	for i, pubCode := range cooked.PubCodes {
		dat := cooked.Data[i]
		fmt.Printf("%s\n", pubCode)
		for j, cnt := range dat {
			n := (cnt * 1024) / cooked.Max
			n = (n * w) / 1024
			day := cooked.Days[j]
			bar := strings.Repeat("*", n)
			wd := weekday(day)
			fmt.Printf("%s %10s %*d %s\n", wd, day, numReserve, cnt, bar)
		}
		fmt.Printf("\n")
	}

}

// Output the summary as a csv file
func dumpCSV(cooked *slurp.CookedSummary) error {

	out := csv.NewWriter(os.Stdout)

	// header
	header := []string{"publication"}
	for _, day := range cooked.Days {
		header = append(header, day)
	}
	out.Write(header)

	//
	for i, pubCode := range cooked.PubCodes {
		dat := cooked.Data[i]
		row := make([]string, len(header))
		row[0] = pubCode
		for j, cnt := range dat {
			row[1+j] = strconv.Itoa(cnt)
		}
		out.Write(row)
	}
	out.Flush()
	return out.Error()
}

// readSites reads the "url" column of csvFile.
// (straight cut & paste from crawl/main.go)
func readSites(csvFile string) ([]string, error) {
	out := []string{}

	f, err := os.Open(csvFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comment = '#'

	headers, err := r.Read()
	if err != nil {
		return nil, err
	}

	urlCol := -1
	//	statusCol := -1
	for col, name := range headers {
		switch strings.ToLower(name) {
		case "url":
			urlCol = col
			break
			//		case "status":
			//			statusCol = col
			//			break
		}
	}
	if urlCol == -1 {
		return nil, errors.New("Missing url column")
	}

	for {
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		url := row[urlCol]

		out = append(out, url)
	}
	return out, nil
}

// urlToPubcodes converts a site url to potential pubcodes which might be used.
// The "prefered" one will be first in the list:
// "https://www.dailyfoobar.com/home" => ["www.dailyfoobar.com", "dailyfoobar.com"]
// "https://dailyfoobar.com" => ["dailyfoobar.com", "www.dailyfoobar.com"]
// "/badurl.html" => []
func urlToPubcodes(siteURL string) []string {
	parsed, err := neturl.Parse(siteURL)
	if err != nil {
		return []string{}
	}
	host := strings.ToLower(parsed.Hostname())
	if strings.HasPrefix(host, "www.") {
		return []string{host, strings.TrimPrefix(host, "www.")}
	} else {
		return []string{host, "www." + host}
	}
}

// addMissingSites adds empty result entries into the raw data for any sites
// which are not already represented.
// Mutates the 'raw' param.
func addMissingSites(raw slurp.RawSummary, siteURLs []string) {
	for _, siteURL := range siteURLs {
		alreadyGot := false
		pubCodes := urlToPubcodes(siteURL)
		if len(pubCodes) < 1 {
			fmt.Fprintf(os.Stderr, "WARN: couldn't get a pubcode from '%s' - ignoring.", siteURL)
			continue
		}
		for _, code := range pubCodes {
			if _, got := raw[code]; got {
				alreadyGot = true
			}
		}

		if !alreadyGot {
			// Add an empty placeholder entry for the "preferred" (first)
			// pubcode.
			raw[pubCodes[0]] = map[string]int{}
		}
	}
}
