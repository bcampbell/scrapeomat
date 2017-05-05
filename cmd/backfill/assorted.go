package main

import (
	"encoding/json"
	"fmt"
	"github.com/bcampbell/arts/util"
	"net/http"
)

// vice section pages are all javascript. There's an api to step back through the articles.
func DoViceUK(opts *Options) error {
	client := &http.Client{
		Transport: util.NewPoliteTripper(),
	}

	type viceArtData struct {
		URL string `json:"url"`
	}
	type viceArt struct {
		Type string      `json:"type"`
		Data viceArtData `json:"data"`
	}

	for page := 1; page < (1 + opts.nPages); page++ {
		u := fmt.Sprintf("https://www.vice.com/api/v1/latest?locale=en_uk&page=%d", page)

		resp, err := client.Get(u)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		/*
			raw, err := ioutil.ReadAll(resp.Body)

			if err != nil {
				return err
			}
		*/
		dec := json.NewDecoder(resp.Body)
		var arts []viceArt

		err = dec.Decode(&arts)
		if err != nil {
			return err
		}
		for _, art := range arts {
			if art.Type != "articles" {
				continue
			}
			fmt.Println(art.Data.URL)
		}

	}

	return nil
}
