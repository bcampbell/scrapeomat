package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"semprini/scrapeomat/store"
	"time"
)

type Logger interface {
	Printf(format string, v ...interface{})
}

type nullLogger struct{}

func (l nullLogger) Printf(format string, v ...interface{}) {
}

type SlurpServer struct {
	ErrLog  Logger
	InfoLog Logger
	Port    int
	Prefix  string

	db *store.Store

	tmpls struct {
		browse *template.Template
	}
}

func New(db *store.Store, port int, prefix string, infoLog Logger, errLog Logger) (*SlurpServer, error) {
	srv := &SlurpServer{db: db, Port: port, Prefix: prefix, InfoLog: infoLog, ErrLog: errLog}

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
	srv.tmpls.browse = t

	return srv, nil
}

func (srv *SlurpServer) Run() error {

	http.HandleFunc(srv.Prefix+"/api/slurp", func(w http.ResponseWriter, r *http.Request) {
		srv.slurpHandler(&Context{}, w, r)
	})
	http.HandleFunc(srv.Prefix+"/browse", func(w http.ResponseWriter, r *http.Request) {
		srv.browseHandler(&Context{}, w, r)
	})

	srv.InfoLog.Printf("Started at localhost:%d%s/\n", srv.Port, srv.Prefix)
	return http.ListenAndServe(fmt.Sprintf(":%d", srv.Port), nil)
}

// for auth etc... one day.
type Context struct {
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

func (srv *SlurpServer) slurpHandler(ctx *Context, w http.ResponseWriter, r *http.Request) {

	filt, err := getFilter(r)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	//	srv.InfoLog.Printf("%+v\n", filt)

	abort := make(chan struct{})
	defer close(abort)

	totalArts, err := srv.db.FetchCount(filt)
	if err != nil {
		// TODO: should send error via json
		http.Error(w, fmt.Sprintf("DB error: %s", err), 500)
		return
	}
	srv.InfoLog.Printf("%d articles to send\n", totalArts)

	sent := 0

	c := srv.db.Fetch(abort, filt)
	for fetched := range c {
		msg := Msg{}
		if fetched.Err == nil {
			msg.Article = fetched.Art
		} else {
			msg.Error = fmt.Sprintf("fetch error: %s\n", fetched.Err)
			srv.ErrLog.Printf(msg.Error)
		}
		outBuf, err := json.Marshal(msg)
		if err != nil {
			srv.ErrLog.Printf("json encoding error: %s\n", err)
			abort <- struct{}{}
			return
		}
		_, err = w.Write(outBuf)
		if err != nil {
			srv.ErrLog.Printf("write error: %s\n", err)
			abort <- struct{}{}
			return
		}

		if fetched.Err == nil {
			sent++
			//	if (sent % 10) == 0 {
			//		fmt.Printf("Sent %d/%d\n", sent, totalArts)
			//	}
		} else {
			return
		}
	}

}

func (srv *SlurpServer) browseHandler(ctx *Context, w http.ResponseWriter, r *http.Request) {
	filt, err := getFilter(r)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	abort := make(chan struct{})
	defer close(abort)

	filt.Limit = 100

	totalArts, err := srv.db.FetchCount(filt)
	if err != nil {
		// TODO: should send error via json
		http.Error(w, fmt.Sprintf("DB error: %s", err), 500)
		return
	}
	srv.InfoLog.Printf("%d articles to send\n", totalArts)

	c := srv.db.Fetch(abort, filt)
	arts := make([]*store.Article, 0)
	for fetched := range c {
		if fetched.Err != nil {
			http.Error(w, fmt.Sprintf("Fetch error: %s", fetched.Err), 500)
			return
		}

		arts = append(arts, fetched.Art)
	}
	/*
		for _, art := range arts {
			fmt.Println(art.Headline)
		}
	*/
	params := struct {
		Arts []*store.Article
	}{
		arts,
	}
	err = srv.tmpls.browse.Execute(w, params)
	if err != nil {
		http.Error(w, fmt.Sprintf("template error: %s", err), 500)
		return
	}
}
