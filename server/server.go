package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"semprini/scrapeomat/store"
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

func handler(ctx *Context, w http.ResponseWriter, r *http.Request) {
	abort := make(chan struct{})
	c := ctx.db.Fetch(abort)

	for fetched := range c {
		if fetched.Err == nil {
			outBuf, err := json.Marshal(fetched.Art)
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
		} else {
			// failed during fetch...
			fmt.Printf("fetch error: %s\n", fetched.Err)
			// TODO: inform client properly!
			w.Write([]byte("POOP!"))
			return
		}

	}

}
