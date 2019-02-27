package main

import (
	"flag"
	"fmt"
	"github.com/bcampbell/scrapeomat/slurp"
	"os"
)

var filtParams struct {
	//	pubFrom, pubTo string
	//	pubCodes       string
	sinceID int
	count   int
}

func init() {
	flag.IntVar(&filtParams.sinceID, "since_id", 0, "only return articles with id>since_id")
	flag.IntVar(&filtParams.count, "count", 0, "max num of articles to return (per http request)")

	//	flag.StringVar(&filtParams.pubFrom, "pubfrom", 0, "only articles published on or after this date")
	//	flag.StringVar(&filtParams.pubTo, "pubto", 0, "only articles published before this date")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS]... <server URL>\n", os.Args[0])
		flag.PrintDefaults()
	}
}

func main() {

	flag.Parse()

	filt := &slurp.Filter{
		Count:   filtParams.count,
		SinceID: filtParams.sinceID,
	}
	client := slurp.NewSlurper(flag.Arg(0))

	incoming, _ := client.Slurp(filt)

	for msg := range incoming {
		if msg.Error != "" {
			fmt.Fprintf(os.Stdout, "ERROR: %s\n", msg.Error)
		} else if msg.Article != nil {
			art := msg.Article
			fmt.Printf("%s (%s)\n", art.Headline, art.CanonicalURL)
		} else {
			fmt.Fprintf(os.Stdout, "WARN: empty message...\n")
		}
	}
}
