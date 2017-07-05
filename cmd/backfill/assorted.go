package main

import (
	"encoding/json"
	"fmt"
	"github.com/andybalholm/cascadia"
	"github.com/bcampbell/arts/util"
	"net/http"
	"net/url"
	"os"
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

// eg:
// https://www.sdpnoticias.com/nacional/list?page=80

func DoSDPNoticias(opts *Options) error {

	sections := []string{
		"nacional", "internacional", "columnas", "deportes", "economia",
		"sorprendente", "tecnologia",
		"geek", "estilo-de-vida",
		"enelshow/television", "enelshow/musica",
		"enelshow/cine", "enelshow/famosos",
		"gay", "sexxion",
		"pitorreo",
		"local/baja-california-sur",
		"local/ciudad-de-mexico",
		"local/chiapas",
		"local/coahuila",
		"local/edomex",
		"local/guadalajara",
		"local/guerrero",
		"local/jalisco",
		"local/monterrey",
		"local/morelos",
		"local/nuevo-leon",
		"local/oaxaca",
		"local/puebla",
		"local/quintana-roo",
		"local/sonora",
		"local/tamaulipas",
		"local/veracruz",
		"estados"}

	for _, section := range sections {
		s := &Searcher{
			SearchURL:     fmt.Sprintf("https://www.sdpnoticias.com/%s/list", section),
			Params:        url.Values{},
			PageParam:     "page",
			ResultLinkSel: cascadia.MustCompile(".news-listing a"),
			NPages:        opts.nPages,
		}

		err := s.Run(os.Stdout)
		if err != nil {

			continue
			//		return err
		}
	}
	return nil
}
