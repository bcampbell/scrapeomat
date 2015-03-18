package main

import (
	"flag"
	"fmt"
	"semprini/scrapeomat/slurp"
)

func main() {

	flag.Parse()

	client := slurp.NewSlurper(flag.Arg(0))

	incoming := client.Slurp("2015-03-10", "2015-03-10")

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
