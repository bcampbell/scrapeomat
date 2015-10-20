package arc

// helpers to write out raw HTTP requests/responses to noddy .warc files

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"
)

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func copyResponse(orig *http.Response) (*http.Response, error) {
	// read body first
	bod, err := ioutil.ReadAll(orig.Body)
	if err != nil {
		return nil, err
	}
	orig.Body.Close()
	orig.Body = nopCloser{bytes.NewReader(bod)}

	clone := *orig
	clone.Body = nopCloser{bytes.NewReader(bod)}

	return &clone, nil
}

// eg "abcdefg.foo" returns "a/ab/acb"
func spreadPath(name string) string {
	numChunks := 3 // how many subdirs to use
	chunkSize := 1 // num chars per subdir

	if len(name) < numChunks*chunkSize {
		panic("name too short")
	}

	parts := make([]string, numChunks)
	for chunk := 0; chunk < numChunks; chunk++ {
		parts[chunk] = name[0 : (chunk+1)*chunkSize]
	}
	return path.Join(parts...)
}

/*
func AlreadyGot(warcDir, srcURL string) bool,error {
	u, err := url.Parse(srcURL)
	if err != nil {
		return err
	}
	hasher := md5.New()
	hasher.Write([]byte(srcURL))
	filename := hex.EncodeToString(hasher.Sum(nil)) + ".warc"
	dir := path.Join(warcDir, u.Host, spreadPath(filename))
    full := path.Join(dir, filename)
}
*/

func ArchiveResponse(warcDir string, resp *http.Response, srcURL string, timeStamp time.Time) error {

	u, err := url.Parse(srcURL)
	if err != nil {
		return err
	}

	hasher := md5.New()
	hasher.Write([]byte(srcURL))
	filename := hex.EncodeToString(hasher.Sum(nil)) + ".warc.gz"

	//dir := path.Join(warcDir, u.Host, timeStamp.UTC().Format("2006-01-02"))

	// .../www.example.com/1/12/123/12345678.warc
	dir := path.Join(warcDir, u.Host, spreadPath(filename))
	err = os.MkdirAll(dir, 0700)
	if err != nil {
		return err
	}

	outfile, err := os.Create(path.Join(dir, filename))
	if err != nil {
		return err
	}
	defer outfile.Close()

	gzw := gzip.NewWriter(outfile)
	defer gzw.Close()

	return WriteWARC(gzw, resp, srcURL, timeStamp)
}

func WriteWARC(w io.Writer, resp *http.Response, srcURL string, timeStamp time.Time) error {

	// copy the response so we can peek at the body
	tmpResp, err := copyResponse(resp)
	if err != nil {
		return err
	}

	var payload bytes.Buffer
	err = tmpResp.Write(&payload)
	if err != nil {
		return err
	}

	warcHdr := http.Header{}
	// required fields
	warcHdr.Set("WARC-Record-ID", fmt.Sprintf("urn:X-scrapeomat:%d", time.Now().UnixNano()))
	warcHdr.Set("Content-Length", fmt.Sprintf("%d", payload.Len()))
	warcHdr.Set("WARC-Date", timeStamp.UTC().Format(time.RFC3339))
	warcHdr.Set("WARC-Type", "response")
	// some extras

	warcHdr.Set("WARC-Target-URI", tmpResp.Request.URL.String())
	// cheesy custom field for original url, in case we were redirected
	warcHdr.Set("X-Scrapeomat-Srcurl", srcURL)
	//	warcHdr.Set("WARC-IP-Address", "")
	warcHdr.Set("Content-Type", "application/http; msgtype=response")

	fmt.Fprintf(w, "WARC/1.0\r\n")
	err = warcHdr.Write(w)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "\r\n")
	_, err = payload.WriteTo(w)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "\r\n")
	fmt.Fprintf(w, "\r\n")

	return nil
}
