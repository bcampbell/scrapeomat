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

	// note: filenamify default length is 100 which is waaaaay too short for us.
	safeName, err := filenamify.Filenamify(u, filenamify.Options{MaxLength: 250})
	cacheName := filepath.Join(cacheDir, safeName)

	resp, err := warc.ReadFile(cacheName)
	if err != nil {
		if os.IsNotExist(err) {
			// not in cache - perform a real http request
			resp, err = client.Get(u)
			if err != nil {
				return nil, err
			}
			cache := false
			// Cache 2xx, 3xx and 4xx responses
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				cache = true
			}
			if resp.StatusCode >= 300 && resp.StatusCode < 400 {
				cache = true
			}
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				cache = true
			}
			if cache {
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
	}
	return resp, nil
}
