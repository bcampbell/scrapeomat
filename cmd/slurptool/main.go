package main

import (
	"flag"
	"fmt"
	"semprini/scrapeomat/slurp"
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
}

func main() {

	flag.Parse()

	filt := &slurp.Filter{
		Count:   filtParams.count,
		SinceID: filtParams.sinceID,
	}
	client := slurp.NewSlurper(flag.Arg(0))

	incoming := client.Slurp(filt)

	for msg := range incoming {
		if msg.Error != "" {
			fmt.Printf("ERROR: %s\n", msg.Error)
		} else if msg.Article != nil {
			art := msg.Article
			fmt.Printf("%s (%s)\n", art.Headline, art.CanonicalURL)
		} else {
			fmt.Printf("WARN empty message...\n")
		}
	}
}
