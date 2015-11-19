package main

import (
	"flag"
	"fmt"
	"os"
	"semprini/scrapeomat/slurp"
	"time"
)

var opts struct {
	server   string
	from, to string
}

func init() {
	flag.StringVar(&opts.from, "from", "", "from date")
	flag.StringVar(&opts.to, "to", "", "to date")
	flag.StringVar(&opts.server, "s", "localhost:13568", "API server to query")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS]\n", os.Args[0])
		flag.PrintDefaults()
	}
}

func main() {

	flag.Parse()

	from, err := time.Parse("2006-01-02", opts.from)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Bad from: %s\n", err)
		os.Exit(2)
	}
	to, err := time.Parse("2006-01-02", opts.to)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Bad to: %s\n", err)
		os.Exit(2)
	}
	slurper := slurp.NewSlurper(opts.server)

	raw, err := slurper.Summary(from, to)

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(2)
	}

	cooked := slurp.CookSummary(raw, opts.from, opts.to)

	dump(cooked)
}

func dump(cooked *slurp.CookedSummary) {

	fmt.Printf("           ")
	for _, day := range cooked.Days {
		fmt.Printf("%10s ", day)
	}
	fmt.Printf("\n")

	for i, pubCode := range cooked.PubCodes {
		fmt.Printf("%10s ", pubCode)
		dat := cooked.Data[i]
		for _, cnt := range dat {
			fmt.Printf("%10d ", cnt)
		}
		fmt.Printf("\n")
	}

}
