package main

import (
	"github.com/bcampbell/warc"
	"github.com/flytam/filenamify"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// HTTPGetWithCache performs a GET, using files in cacheDir to cache requests.
// If cacheDir is "", don't bother caching.
func HTTPGetWithCache(client *http.Client, u string, cacheDir string) (*http.Response, error) {

	// passthru if we're not using a cache at all
	if cacheDir == "" {
		return client.Get(u)
	}
	err := os.MkdirAll(cacheDir, os.ModePerm)
	if err != nil {
		return nil, err
	}

	safeName, err := filenamify.Filenamify(u, filenamify.Options{})
	cacheName := filepath.Join(cacheDir, safeName)

	resp, err := warc.ReadFile(cacheName)
	if err != nil {
		if os.IsNotExist(err) {
			// not in cache - perform a real http request
			resp, err = client.Get(u)
			if err != nil {
				return nil, err
			}
			// success. write to cache.
			out, err := os.Create(cacheName)
			if err != nil {
				return nil, err
			}
			err = warc.Write(out, resp, u, time.Now())
			if err != nil {
				return nil, err
			}
		}
	}
	return resp, nil
}
