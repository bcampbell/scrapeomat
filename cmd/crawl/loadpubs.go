package main

import (
	"github.com/bcampbell/scrapeomat/store"
	neturl "net/url"
)

func loadPubs(db store.Store, siteURLs []string) error {
	pubs := []*store.Publication{}
	for _, siteURL := range siteURLs {
		u, err := neturl.Parse(siteURL)
		if err != nil {
			return err
		}

		pubs = append(pubs, &store.Publication{
			Domain: u.Hostname(),
			Code:   u.Hostname(),
		})
	}

	_, err := db.FindOrAddPublications(pubs...)
	if err != nil {
		return err
	}

	return nil
}
