package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/handlers"
	"html/template"
	"net/http"
	"semprini/scrapeomat/store"
	"strconv"
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

func NewServer(db *store.Store, port int, prefix string, infoLog Logger, errLog Logger) (*SlurpServer, error) {
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

	http.Handle(srv.Prefix+"/api/slurp", handlers.CompressHandler(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				srv.slurpHandler(&Context{}, w, r)
			})))

	http.Handle(srv.Prefix+"/api/pubs", handlers.CompressHandler(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				srv.pubsHandler(&Context{}, w, r)
			})))

	http.HandleFunc(srv.Prefix+"/api/count", func(w http.ResponseWriter, r *http.Request) {
		srv.countHandler(&Context{}, w, r)
	})
	/*
		http.HandleFunc(srv.Prefix+"/browse", func(w http.ResponseWriter, r *http.Request) {
			srv.browseHandler(&Context{}, w, r)
		})
	*/

	srv.InfoLog.Printf("Started at localhost:%d%s/\n", srv.Port, srv.Prefix)
	return http.ListenAndServe(fmt.Sprintf(":%d", srv.Port), nil)
}

// for auth etc... one day.
type Context struct {
}

type Msg struct {
	Article *store.Article `json:"article,omitempty"`
	Error   string         `json:"error,omitempty"`
	Next    struct {
		SinceID int `json:"since_id,omitempty"`
	} `json:"next,omitempty"`
	/*
		Info    struct {
			Sent  int
			Total int
		} `json:"info,omitempty"`
	*/
}

func parseTime(in string) (time.Time, error) {

	t, err := time.ParseInLocation(time.RFC3339, in, time.UTC)
	if err == nil {
		return t, nil
	}

	// short form - assumes you want utc days rather than local days...
	const dateOnlyFmt = "2006-01-02"
	t, err = time.ParseInLocation(dateOnlyFmt, in, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date/time format")
	}

	return t, nil

}

func getFilter(r *http.Request) (*store.Filter, error) {
	maxCount := 20000

	filt := &store.Filter{}

	// deprecated!
	if r.FormValue("from") != "" {
		t, err := parseTime(r.FormValue("from"))
		if err != nil {
			return nil, fmt.Errorf("bad 'from' param")
		}

		filt.PubFrom = t
	}

	// deprecated!
	if r.FormValue("to") != "" {
		t, err := parseTime(r.FormValue("to"))
		if err != nil {
			return nil, fmt.Errorf("bad 'to' param")
		}
		t = t.AddDate(0, 0, 1) // add one day
		filt.PubTo = t
	}

	if r.FormValue("pubfrom") != "" {
		t, err := parseTime(r.FormValue("pubfrom"))
		if err != nil {
			return nil, fmt.Errorf("bad 'pubfrom' param")
		}

		filt.PubFrom = t
	}
	if r.FormValue("pubto") != "" {
		t, err := parseTime(r.FormValue("pubto"))
		if err != nil {
			return nil, fmt.Errorf("bad 'pubto' param")
		}

		filt.PubTo = t
	}

	if r.FormValue("since_id") != "" {
		sinceID, err := strconv.Atoi(r.FormValue("since_id"))
		if err != nil {
			return nil, fmt.Errorf("bad 'since_id' param")
		}
		if sinceID > 0 {
			filt.SinceID = sinceID
		}
	}

	if r.FormValue("count") != "" {
		cnt, err := strconv.Atoi(r.FormValue("count"))
		if err != nil {
			return nil, fmt.Errorf("bad 'count' param")
		}
		filt.Count = cnt
	} else {
		// default to max
		filt.Count = maxCount
	}

	// enforce max count
	if filt.Count > maxCount {
		return nil, fmt.Errorf("'count' too high (max %d)", maxCount)
	}

	// publication codes?
	if pubs, got := r.Form["pub"]; got {
		filt.PubCodes = pubs
	}

	// publication codes to exclude?
	if xpubs, got := r.Form["xpub"]; got {
		filt.XPubCodes = xpubs
	}

	return filt, nil
}

// implement the main article slurp API
func (srv *SlurpServer) slurpHandler(ctx *Context, w http.ResponseWriter, r *http.Request) {

	filt, err := getFilter(r)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	//	srv.InfoLog.Printf("%+v\n", filt)

	/*
		totalArts, err := srv.db.FetchCount(filt)
		if err != nil {
			// TODO: should send error via json
			http.Error(w, fmt.Sprintf("DB error: %s", err), 500)
			return
		}
		srv.InfoLog.Printf("%d articles to send\n", totalArts)
	*/

	err, artCnt, byteCnt := srv.performSlurp(w, filt)
	status := "OK"
	if err != nil {
		status = fmt.Sprintf("FAIL (%s)", err)
	}

	srv.InfoLog.Printf("%s %s %d arts %d bytes %s\n", r.RemoteAddr, status, artCnt, byteCnt, filt.Describe())
}

// helper fn
func writeMsg(w http.ResponseWriter, msg *Msg) (int, error) {
	outBuf, err := json.Marshal(msg)
	if err != nil {
		return 0, fmt.Errorf("json encoding error: %s", err)
	}
	n, err := w.Write(outBuf)
	if err != nil {
		return n, fmt.Errorf("write error: %s", err)
	}

	return n, nil
}

func (srv *SlurpServer) performSlurp(w http.ResponseWriter, filt *store.Filter) (error, int, int) {

	artCnt := 0
	byteCnt := 0
	c, abort := srv.db.Fetch(filt)
	maxID := 0
	for fetched := range c {
		if fetched.Art != nil {
			msg := Msg{Article: fetched.Art}
			n, err := writeMsg(w, &msg)
			if err != nil {
				abort <- struct{}{}
				return err, artCnt, byteCnt
			}
			byteCnt += n
			artCnt++
			if fetched.Art.ID > maxID {
				maxID = fetched.Art.ID
			}
		}

		if fetched.Err != nil {
			// uhoh - some sort of database error... log and send it on to the client
			msg := Msg{Error: fmt.Sprintf("fetch error: %s\n", fetched.Err)}
			srv.ErrLog.Printf("%s\n", msg.Error)
			n, err := writeMsg(w, &msg)
			if err != nil {
				abort <- struct{}{}
				return err, artCnt, byteCnt
			}
			byteCnt += n
		}
	}

	// looks like more articles to fetch?
	if artCnt == filt.Count {
		// send a "Next" message with a new since_id
		msg := Msg{}
		msg.Next.SinceID = maxID
		n, err := writeMsg(w, &msg)
		if err != nil {
			abort <- struct{}{}
			return err, artCnt, byteCnt
		}
		byteCnt += n
	}

	return nil, artCnt, byteCnt
}

func (srv *SlurpServer) browseHandler(ctx *Context, w http.ResponseWriter, r *http.Request) {
	filt, err := getFilter(r)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	filt.Count = 100

	totalArts, err := srv.db.FetchCount(filt)
	if err != nil {
		// TODO: should send error via json
		http.Error(w, fmt.Sprintf("DB error: %s", err), 500)
		return
	}
	srv.InfoLog.Printf("%d articles to send\n", totalArts)

	c, _ := srv.db.Fetch(filt)
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

// implement the publication list API
func (srv *SlurpServer) pubsHandler(ctx *Context, w http.ResponseWriter, r *http.Request) {

	pubs, err := srv.db.FetchPublications()
	if err != nil {
		http.Error(w, fmt.Sprintf("DB error: %s", err), 500)
		return
	}

	out := struct {
		Publications []store.Publication `json:"publications"`
	}{
		pubs,
	}

	outBuf, err := json.Marshal(out)
	if err != nil {
		http.Error(w, fmt.Sprintf("json encoding error: %s", err), 500)
		return
	}
	_, err = w.Write(outBuf)
	if err != nil {
		srv.ErrLog.Printf("Write error: %s\n", err)
		return
	}

	srv.InfoLog.Printf("%s publications\n", r.RemoteAddr)
}
