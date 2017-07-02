package main

import (
	"fmt"
	"github.com/andybalholm/cascadia"
	"github.com/bcampbell/arts/util"
	"net/http"
	"os"
)

// archive pages, form:
// http://www.jornada.unam.mx/YYYY/MM/DD/section

func DoJornada(opts *Options) error {

	linkSel := cascadia.MustCompile(`#section-cont a`)

	sections := []string{"", "opinion", "politica", "economia", "mundo", "estados", "capital", "sociedad", "deportes", "cultura", "espectaculos"}

	days, err := opts.DayRange()
	if err != nil {
		return err
	}

	client := &http.Client{Transport: util.NewPoliteTripper()}

	for _, day := range days {
		for _, section := range sections {
			u := fmt.Sprintf("http://www.jornada.unam.mx/%04d/%02d/%02d/%s", day.Year(), day.Month(), day.Day(), section)

			doc, err := fetchAndParse(client, u)
			if err != nil {
				fmt.Fprintf(os.Stderr, "SKIP: %s\n", err)
				continue
			}

			links, err := grabLinks(doc, linkSel, u)
			if err != nil {
				return err
			}

			for _, l := range links {
				fmt.Println(l)
			}

		}
		// explicitly add the per-day editorials
		fmt.Printf("http://www.jornada.unam.mx/%04d/%02d/%02d/edito\n", day.Year(), day.Month(), day.Day())
		fmt.Printf("http://www.jornada.unam.mx/%04d/%02d/%02d/correo\n", day.Year(), day.Month(), day.Day())
	}
	return nil
}
