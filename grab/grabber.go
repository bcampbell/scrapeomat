package grab

import (
	"context"
	"fmt"
	"github.com/bcampbell/arts/util"
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// Grabber is an interface to abstract out basic fetching of web pages.
type Grabber interface {
	// Grab performs an HTTP GET and returns the response (or an error).
	// Non-200 HTTP responses are considered to be errors too (unlike
	// http.Client).
	Grab(ctx context.Context, url string) (http.Header, []byte, error)
}

// DefaultGrabber is a grabber which uses the stdlib http.Client.
type DefaultGrabber struct {
	Client *http.Client
}

func NewDefaultGrabber(cacheDir string) (*DefaultGrabber, error) {
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
	return &DefaultGrabber{Client: client}, nil
}

func (g *DefaultGrabber) Grab(ctx context.Context, url string) (http.Header, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/115.0")
	req.Header.Set("Accept", "*/*")

	//	req.Header.Set("User-Agent", `Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:28.0) Gecko/20100101 Firefox/28.0`)
	//	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	//	req.Header.Set("Accept-Encoding", "gzip, deflate, br")

	//	req.Header.Set("Connection", "keep-alive")
	//	req.Header.Set("Upgrade-Insecure-Requests", "1")
	//	req.Header.Set("Sec-Fetch-Dest", "document")
	//	req.Header.Set("Sec-Fetch-Mode", "navigate")
	//	req.Header.Set("Sec-Fetch-Site", "cross-site")
	//	req.Header.Set("TE", "trailers")

	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("HTTP error %s", resp.Status)
	}

	// read in the body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	return resp.Header, body, nil
}

// for external grabber, use textproto.ReadMIMEHeader() to parse headers?
// or just return empty headers!
//
// curl can dump response headers to separate file with -D:
// $ curl -f -D hdrfile <URL>
//

// CurlGrabber is a grabber which calls commandline curl to fetch the page.
type CurlGrabber struct {
	lastGrab time.Time
}

func NewCurlGrabber() (*CurlGrabber, error) {
	return &CurlGrabber{}, nil
}

func (g *CurlGrabber) Grab(ctx context.Context, url string) (http.Header, []byte, error) {
	pause := time.Until(g.lastGrab.Add(1 * time.Second))
	if pause > 0 {
		time.Sleep(pause)
	}
	g.lastGrab = time.Now()

	ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	agentHeader := "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.3"

	cmd := exec.CommandContext(ctx2, "curl", "-f", "-H", agentHeader, url)
	body, err := cmd.Output()

	if err != nil {
		return nil, nil, err
		// This will fail after 100 milliseconds. The 5 second sleep
		// will be interrupted.
	}

	// We _could_ save out the headers to a file ("-h filename") then
	// parse them here, but we don't really need the headers just now.
	return http.Header{}, body, nil
}
