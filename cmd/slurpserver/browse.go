package main

import (
	"fmt"
	"github.com/bcampbell/scrapeomat/store"
	"html/template"
	"net/http"
	"os"
	"strconv"
)

func (srv *SlurpServer) browseHandler(w http.ResponseWriter, r *http.Request) {

	ctx := &Context{Prefix: srv.Prefix}

	filt, err := getFilter(r)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	filt.Count = 100

	pubs, err := srv.db.FetchPublications()
	if err != nil {
		http.Error(w, fmt.Sprintf("FetchPublications error: %s", err), 500)
		return
	}

	c, _ := srv.db.Fetch(filt)
	arts := make([]*store.Article, 0)
	for fetched := range c {
		if fetched.Err != nil {
			http.Error(w, fmt.Sprintf("Fetch error: %s", fetched.Err), 500)
			return
		}

		arts = append(arts, fetched.Art)
	}

	//
	highID := 0
	for _, art := range arts {
		if art.ID > highID {
			highID = art.ID
		}
	}

	// set up url to grab next batch
	nextFilt := Filter(*filt)
	nextFilt.SinceID = highID
	nextFilt.Count = 0

	params := struct {
		Ctx     *Context
		Filt    *Filter
		Pubs    []store.Publication
		Arts    []*store.Article
		MoreURL template.URL
	}{
		ctx,
		(*Filter)(filt),
		pubs,
		arts,
		template.URL("?" + nextFilt.Params().Encode()),
	}

	err = srv.tmpls.browse.Execute(w, params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "template error: %s", err)
		return
	}
}

// display a single article
func (srv *SlurpServer) artHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("id=%s\n", r.FormValue("id"))
	if r.FormValue("id") == "" {
		http.Error(w, "Not found", 404)
		return
	}

	artID, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		http.Error(w, "Not found", 404)
		return
	}

	art, err := srv.db.FetchArt(artID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	ctx := &Context{Prefix: srv.Prefix}

	params := struct {
		Ctx *Context
		Art *store.Article
	}{
		ctx,
		art,
	}

	err = srv.tmpls.art.Execute(w, params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "template error: %s", err)
		return
	}
}
