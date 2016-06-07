package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"semprini/scrapeomat/store"
)

var baseTmpl string = `
<!DOCTYPE html>
<html>
  <head>
    <title>scrapeomat</title>
  </head>
<body>
<header class="site-header">
</header>
{{ template "body" . }}
</body>
</html>
`

var browseTmpl string = `
{{define "body"}}
<form action="" method="GET">
  <label for="pubfrom" >PubFrom:</label>
  <input id="pubfrom" name="pubfrom" type="date" value="{{.Filt.PubFromString}}" />
  <label for="pubto" >PubTo:</label>
  <input id="pubto" name="pubto" type="date" value="{{.Filt.PubToString}}" />
  <select id="pub" name="pub" multiple>
    {{ $f := .Filt}}
    {{ range .Pubs }}
    <option value="{{.Code}}"{{if $f.IsPubSet .Code}} selected{{end}}>{{.Code}}</option>
    {{end}}
  </select>
<input type="submit" value="submit" />
</form>
  <table>
    {{range .Arts}}
    <tr>
        <td>{{.ID}}</td>
        <td><a href="{{.CanonicalURL}}">{{.Headline}}</td>
        <td>{{.Publication.Code}}</td>
        <td>{{.Published}}</td>
        <td>{{range .Authors}}{{.Name}}, {{end}}</td>
    </tr>
    {{end}}
  </table>
  <a href="{{.MoreURL}}">more...</a>

{{end}}
`

func (srv *SlurpServer) browseHandler(ctx *Context, w http.ResponseWriter, r *http.Request) {
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
		Filt    *Filter
		Pubs    []store.Publication
		Arts    []*store.Article
		MoreURL template.URL
	}{
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
