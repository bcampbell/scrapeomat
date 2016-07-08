package main

import (
	"fmt"
	"github.com/andybalholm/cascadia"
	"github.com/bcampbell/arts/util"
	"net/http"
)

// archive pages, form:
// http://www.telegraph.co.uk/archive/2009-2-15.html

func DoTelegraph(opts *Options) error {

	linkSel := cascadia.MustCompile(`.summary h3 a`)

	days, err := opts.DayRange()
	if err != nil {
		return err
	}

	client := &http.Client{Transport: util.NewPoliteTripper()}

	for _, day := range days {
		u := fmt.Sprintf("http://www.telegraph.co.uk/archive/%d-%d-%d.html", day.Year(), day.Month(), day.Day())

		doc, err := fetchAndParse(client, u)
		if err != nil {
			return err
		}

		links, err := grabLinks(doc, linkSel, u)
		if err != nil {
			return err
		}

		for _, l := range links {
			fmt.Println(l)
		}
	}
	return nil
}
