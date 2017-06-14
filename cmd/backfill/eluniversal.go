package main

import (
	"fmt"
	"github.com/andybalholm/cascadia"
	"github.com/bcampbell/arts/util"
	"net/http"
)

// use search page
// http://activo.eluniversal.com.mx/historico/search/index.php?q=una&start=0
// returns 20 articles per page
// 'start' param is article number (0-based)

func DoElUniversal(opts *Options) error {

	linkSel := cascadia.MustCompile(`.moduloNoticia .HeadNota a`)

	client := &http.Client{Transport: util.NewPoliteTripper()}

	for n := opts.nStart; n < (opts.nStart + (opts.nPages * 20)); n += 20 {
		u := fmt.Sprintf("http://activo.eluniversal.com.mx/historico/search/index.php?q=una&start=%d", n)

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
