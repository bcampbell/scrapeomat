package arc

// helpers to write out raw HTTP requests/responses to noddy .warc files

import (
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"github.com/bcampbell/warc"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"
)

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
	err = os.MkdirAll(dir, 0777) // let umask cull the perms down...
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

	return warc.Write(gzw, resp, srcURL, timeStamp)
}
