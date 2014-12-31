package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"semprini/scrapeomat/store"
	"time"
)

var tmpls struct {
	browse *template.Template
}

func Run(db store.Store, port int) error {
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
  <table>
    {{range .Arts}}
    <tr>
        <td>{{.Headline}}</td>
        <td>{{.CanonicalURL}}</td>
        <td>{{.Published}}</td>
    </tr>
    {{end}}
  </table>
{{end}}
`

	t := template.New("browse")
	t.Parse(baseTmpl)
	t.Parse(browseTmpl)
	tmpls.browse = t

	http.HandleFunc("/all", func(w http.ResponseWriter, r *http.Request) {
		slurpHandler(&Context{db: db}, w, r)
	})
	http.HandleFunc("/browse", func(w http.ResponseWriter, r *http.Request) {
		browseHandler(&Context{db: db}, w, r)
	})

	return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

type Context struct {
	db store.Store
}

type Msg struct {
	Article *store.Article `json:"article,omitempty"`
	Error   string         `json:"error,omitempty"`
	/*
		Info    struct {
			Sent  int
			Total int
		} `json:"info,omitempty"`
	*/
}

func getFilter(r *http.Request) (*store.Filter, error) {
	const dateFmt = "2006-01-02"
	from, err := time.Parse(dateFmt, r.FormValue("from"))
	if err != nil {
		return nil, fmt.Errorf("bad/missing 'from' param")
	}
	to, err := time.Parse(dateFmt, r.FormValue("to"))
	if err != nil {
		return nil, fmt.Errorf("bad/missing 'to' param")
	}
	to = to.AddDate(0, 0, 1) // add one day

	filt := &store.Filter{PubFrom: from, PubTo: to}
	return filt, nil
}

func slurpHandler(ctx *Context, w http.ResponseWriter, r *http.Request) {

	filt, err := getFilter(r)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	abort := make(chan struct{})
	defer close(abort)
	fmt.Printf("Start fetch request\n")

	totalArts, err := ctx.db.FetchCount(filt)
	if err != nil {
		// TODO: should send error via json
		http.Error(w, fmt.Sprintf("DB error: %s", err), 500)
		return
	}
	fmt.Printf("%d articles to send\n", totalArts)

	sent := 0

	c := ctx.db.Fetch(abort, filt)
	for fetched := range c {
		msg := Msg{}
		if fetched.Err == nil {
			msg.Article = fetched.Art
		} else {
			msg.Error = fmt.Sprintf("fetch error: %s\n", fetched.Err)
			fmt.Println(msg.Error)
		}
		outBuf, err := json.Marshal(msg)
		if err != nil {
			fmt.Printf("json encoding error: %s\n", err)
			abort <- struct{}{}
			return
		}
		_, err = w.Write(outBuf)
		if err != nil {
			fmt.Printf("write error: %s\n", err)
			abort <- struct{}{}
			return
		}

		if fetched.Err == nil {
			sent++
			if (sent % 10) == 0 {
				fmt.Printf("Sent %d/%d\n", sent, totalArts)
			}
		} else {
			return
		}
	}

}

func browseHandler(ctx *Context, w http.ResponseWriter, r *http.Request) {
	fmt.Printf("browseHandler\n")
	filt, err := getFilter(r)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	abort := make(chan struct{})
	defer close(abort)

	filt.Limit = 100

	totalArts, err := ctx.db.FetchCount(filt)
	if err != nil {
		// TODO: should send error via json
		http.Error(w, fmt.Sprintf("DB error: %s", err), 500)
		return
	}
	fmt.Printf("%d articles to send\n", totalArts)

	c := ctx.db.Fetch(abort, filt)
	arts := make([]*store.Article, 0)
	for fetched := range c {
		if fetched.Err != nil {
			http.Error(w, fmt.Sprintf("Fetch error: %s", fetched.Err), 500)
			return
		}

		arts = append(arts, fetched.Art)
	}

	for _, art := range arts {
		fmt.Println(art.Headline)
	}

	params := struct {
		Arts []*store.Article
	}{
		arts,
	}
	err = tmpls.browse.Execute(w, params)
	if err != nil {
		http.Error(w, fmt.Sprintf("template error: %s", err), 500)
		return
	}
}
