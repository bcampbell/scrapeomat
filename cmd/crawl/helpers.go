package main

import (
	"context"
	"github.com/bcampbell/arts/util"
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"net/http"
	"os"
	"time"
)

func buildClient(cacheDir string) (*http.Client, error) {

	polite := util.NewPoliteTripper()
	polite.PerHostDelay = 1 * time.Second

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	if cacheDir == "" {
		client.Transport = polite
	} else {
		err := os.MkdirAll(cacheDir, 0777)
		if err != nil {
			return nil, err
		}
		cache := diskcache.New(cacheDir)
		//fmt.Printf("CACHEDIR: %s\n", cacheDir)
		caching := httpcache.NewTransport(cache)
		caching.Transport = polite

		client.Transport = caching
	}
	return client, nil
}

func buildRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", `Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:28.0) Gecko/20100101 Firefox/28.0`)
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	return req, nil
}
