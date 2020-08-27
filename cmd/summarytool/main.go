package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"github.com/bcampbell/scrapeomat/slurp"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"strconv"
	"strings"
	"time"
)

var opts struct {
	server    string
	from, to  string
	pubs      pubArgs
	termWidth int
	csv       bool
}

type pubArgs []string

func (p *pubArgs) String() string         { return fmt.Sprintf("%s", *p) }
func (p *pubArgs) Set(value string) error { *p = append(*p, value); return nil }

func init() {
	flag.StringVar(&opts.from, "from", "", "from date")
	flag.StringVar(&opts.to, "to", "", "to date")
	flag.IntVar(&opts.termWidth, "w", 0, "output width (0=auto)")
	flag.StringVar(&opts.server, "s", "http://localhost:12345", "`url` of API server to query")
	flag.BoolVar(&opts.csv, "c", false, "output as csv rather than ascii-art")
	flag.Var(&opts.pubs, "p", "publication code(s)")

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

	slurper := slurp.NewSlurper(opts.server)

	raw, err := slurper.Summary(&filt)

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(2)
	}

	cooked := slurp.CookSummary(raw)

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
