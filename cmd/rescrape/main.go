package main

// rescrape is a tool which goes through a directory of .warc files
// scrapes articles from them and loads those articles into
// the scrapeomat store.
// It'll descend into subdirectories as it searches for .warc files.
// Uses multiple CPU cores if available.
//
// caveats:
// it assumes that each .warc file contains a simple request/response
// arrangment and doesn't (yet) do anything clever to collect redirects.
// The initial purpose is to rescrape using the simple .warc files archived
// by scrapeomat.
// Needs some work to generalise it to more complicated .warc arrangements.

//
// TODO:
// use scraper configs to apply URL rejection rules + whatever other metadata (eg publication codes)
import (
	"bufio"
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"github.com/bcampbell/arts/arts"
	"github.com/bcampbell/warc/warc"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"semprini/scrapeomat/store"
	"strings"
	"sync"
)

func worker(db *store.Store, fileChan chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	for warcFile := range fileChan {
		process(db, warcFile)
	}
}

// scrape a .warc file, stash result in db
func process(db *store.Store, f string) {
	scraped, err := fromWARC(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s FAILED: %s\n", f, err)
		return
	}

	// store in database
	//fmt.Printf("stash %s: %v", f, art.URLs)

	art := store.ConvertArticle(scraped)

	//	fmt.Println(art.Published)

	artIDs, err := db.FindURLs(art.URLs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: FindArticle() FAILED: %s\n", f, err)
		return
	}

	if len(artIDs) > 1 {
		fmt.Fprintf(os.Stderr, "%s: multiple articles matching IDs: %v\n", art.URLs, artIDs)
	}

	alreadyGot := (len(artIDs) > 0)
	if alreadyGot && !opts.forceReplace {
		fmt.Fprintf(os.Stderr, "got %s already (id %d)\n", art.URLs[0], artIDs)
		return
	}

	if alreadyGot {
		// force replacement!
		art.ID = artIDs[0]
	}

	artID, err := db.Stash(art)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s stash FAILED: %s\n", f, err)
		return
	}
	if alreadyGot {
		fmt.Fprintf(os.Stdout, "%s : RESCRAPE %d '%s'\n", f, artID, art.Headline)
	} else {
		fmt.Fprintf(os.Stdout, "%s : %d '%s'\n", f, artID, art.Headline)
	}
}

func findWarcFiles(start string) ([]string, error) {
	files := []string{}
	err := filepath.Walk(start, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(path, ".warc") || strings.HasSuffix(path, ".warc.gz") {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

var opts struct {
	databaseURL  string
	forceReplace bool
}

func openStore(connStr string) (*store.Store, error) {
	if connStr == "" {
		connStr = os.Getenv("SCRAPEOMAT_DB")
	}

	if connStr == "" {
		return nil, fmt.Errorf("no database specified (use -db flag or set $SCRAPEOMAT_DB)")
	}

	db, err := store.NewStore(connStr)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: rescrape [options] <path-to-warc-files>\n")
		flag.PrintDefaults()
		os.Exit(2)
	}

	flag.StringVar(&opts.databaseURL, "db", "", "database connection `string` (eg postgres://scrapeomat:password@localhost/scrapeomat)")
	flag.BoolVar(&opts.forceReplace, "f", false, "force replacement of articles already in db")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "ERROR: missing <path-to-warc-files>\n")
		os.Exit(1)
	}

	db, err := openStore(opts.databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
	defer db.Close()

	var wg sync.WaitGroup

	runtime.GOMAXPROCS(runtime.NumCPU())

	files, err := findWarcFiles(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR while finding .warc files: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("MAXPROCS=%d dir=%s %d files\n", runtime.GOMAXPROCS(0), flag.Arg(0), len(files))

	//files := flag.Args()

	// create workers
	fileChan := make(chan string)
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go worker(db, fileChan, &wg)
	}

	// feed the workers
	for _, warcFile := range files {
		fileChan <- warcFile
	}

	close(fileChan)
	wg.Wait()
}

// TODO: this is from arts/scrapetool. Make sure to replicate any improvements there.
func fromWARC(filename string) (*arts.Article, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var in io.Reader
	if filepath.Ext(filename) == ".gz" {
		gin, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gin.Close()
		in = gin
	} else {
		in = f
	}

	warcReader := warc.NewReader(in)
	for {
		//	fmt.Printf("WARC\n")
		rec, err := warcReader.ReadRecord()
		if err != nil {
			return nil, fmt.Errorf("Error reading %s: %s", filename, err)
		}
		if rec.Header.Get("Warc-Type") != "response" {
			continue
		}
		reqURL := rec.Header.Get("Warc-Target-Uri")
		// parse response, grab raw html
		rdr := bufio.NewReader(bytes.NewReader(rec.Block))
		response, err := http.ReadResponse(rdr, nil)
		if err != nil {
			return nil, fmt.Errorf("Error parsing response: %s", err)
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			return nil, fmt.Errorf("HTTP error: %d", response.StatusCode)
		}
		rawHTML, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}
		// TODO: arts should allow passing in raw response? or header + body?
		return arts.ExtractFromHTML(rawHTML, reqURL)
	}

}
