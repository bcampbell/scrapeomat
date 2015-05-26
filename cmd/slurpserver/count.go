package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	//	"semprini/scrapeomat/store"
)

type ArticleCountResult struct {
	ArticleCount int `json:"article_count"`
}

// implement api/count
func (srv *SlurpServer) countHandler(ctx *Context, w http.ResponseWriter, r *http.Request) {

	filt, err := getFilter(r)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	totalArts, err := srv.db.FetchCount(filt)
	if err != nil {
		// TODO: should send error via json?
		http.Error(w, fmt.Sprintf("DB error: %s", err), 500)
		return
	}

	msg := ArticleCountResult{ArticleCount: totalArts}
	outBuf, err := json.Marshal(msg)
	if err != nil {
		errMsg := fmt.Sprintf("json encoding error: %s\n", err)
		srv.ErrLog.Printf(errMsg)
		http.Error(w, errMsg, 500)
		return
	}
	_, err = w.Write(outBuf)
	if err != nil {
		srv.ErrLog.Printf("write error: %s\n", err)
		return
	}

	srv.InfoLog.Printf("%s /api/count OK %d arts %s\n", r.RemoteAddr, totalArts, filt.Describe())
}
