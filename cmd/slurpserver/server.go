package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/handlers"
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

	db           *store.Store
	enableBrowse bool
	tmpls        struct {
		browse *template.Template
	}
}

func NewServer(db *store.Store, enableBrowse bool, port int, prefix string, infoLog Logger, errLog Logger) (*SlurpServer, error) {
	srv := &SlurpServer{db: db, enableBrowse: enableBrowse, Port: port, Prefix: prefix, InfoLog: infoLog, ErrLog: errLog}

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

	http.Handle(srv.Prefix+"/api/summary", handlers.CompressHandler(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				srv.summaryHandler(&Context{}, w, r)
			})))

	http.HandleFunc(srv.Prefix+"/api/count", func(w http.ResponseWriter, r *http.Request) {
		srv.countHandler(&Context{}, w, r)
	})

	if srv.enableBrowse {
		http.HandleFunc(srv.Prefix+"/browse", func(w http.ResponseWriter, r *http.Request) {
			srv.browseHandler(&Context{}, w, r)
		})
	}
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

// implement the summary API
func (srv *SlurpServer) summaryHandler(ctx *Context, w http.ResponseWriter, r *http.Request) {

	var from, to time.Time
	var err error

	var argErr error

	if r.FormValue("from") != "" {
		from, err = parseTime(r.FormValue("from"))
		if err != nil {
			argErr = fmt.Errorf("bad 'from' param")
		}
	} else {
		argErr = fmt.Errorf("missing 'from' param")
	}

	if r.FormValue("to") != "" {
		to, err = parseTime(r.FormValue("to"))
		if err != nil {
			argErr = fmt.Errorf("bad 'to' param")
		}
	} else {
		argErr = fmt.Errorf("missing 'to' param")
	}

	if argErr != nil {
		srv.ErrLog.Printf("ERR: %s\n", argErr)
		http.Error(w, argErr.Error(), 400)
		return
	}

	rawCounts, err := srv.db.FetchSummary(from, to)

	cooked := make(map[string]map[string]int)

	for _, raw := range rawCounts {
		mm, ok := cooked[raw.PubCode]
		if !ok {
			mm = make(map[string]int)
			cooked[raw.PubCode] = mm
		}
		day := raw.Date.Format("2006-01-02")
		mm[day] = raw.Count
	}

	out := struct {
		Counts map[string]map[string]int `json:"counts"`
	}{
		cooked,
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

	srv.InfoLog.Printf("%s summary (%d rows)\n", r.RemoteAddr, len(rawCounts))
}
