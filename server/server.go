package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"semprini/scrapeomat/store"
	"time"
)

func Run(db store.Store, port int) error {
	http.HandleFunc("/all", func(w http.ResponseWriter, r *http.Request) {
		handler(&Context{db: db}, w, r)
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

func handler(ctx *Context, w http.ResponseWriter, r *http.Request) {

	const dateFmt = "2006-01-02"
	from, err := time.Parse(dateFmt, r.FormValue("from"))
	if err != nil {
		http.Error(w, "bad/missing 'from' param", 400)
		return
	}
	to, err := time.Parse(dateFmt, r.FormValue("to"))
	if err != nil {
		http.Error(w, "bad/missing 'to' param", 400)
		return
	}

	abort := make(chan struct{})
	defer close(abort)
	fmt.Printf("Start fetch request: %s...%s\n", from, to)
	totalArts, err := ctx.db.FetchCount(from, to)
	if err != nil {
		// TODO: should send error via json
		http.Error(w, fmt.Sprintf("DB error: %s", err), 500)
		return
	}
	fmt.Printf("%d articles to send\n", totalArts)

	sent := 0

	c := ctx.db.Fetch(abort, from, to)
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
