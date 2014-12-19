package main

// rescrape is a tool which goes through a directory of .warc files
// scrapes articles from them and loads those articles into
// the scrapeomat store.
// It'll descend into subdirectories as it searches for .warc files.
//
// caveats:
// it assumes that each .warc file contains a simple request/response
// arrangment and doesn't (yet) do anything clever to collect redirects.
// The initial purpose is to rescrape using the simple .warc files archived
// by scrapeomat.
// Needs some work to generalise it to more complicated .warc arrangements.

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"github.com/bcampbell/arts/arts"
	"github.com/bcampbell/warc/warc"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"semprini/scrapeomat/store"
	"sync"
)

func worker(db store.Store, fileChan chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	for warcFile := range fileChan {
		process(db, warcFile)
	}
}

// scrape a .warc file, stash result in db
func process(db store.Store, f string) {
	scraped, err := fromWARC(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s FAILED: %s\n", f, err)
		return
	}

	// store in database
	//fmt.Printf("stash %s: %v", f, art.URLs)

	// FUDGE!
	art := store.ConvertArticle(scraped)
	art.Publication.Code = "thetimes"

	fmt.Println(art.Published)

	urls, err := db.WhichAreNew(art.URLs)
	if len(urls) != len(art.URLs) {
		fmt.Fprintf(os.Stderr, "got %s already\n", art.URLs[0])
		return
	}

	_, err = db.Stash(art)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s stash FAILED: %s\n", f, err)
		return
	}
	fmt.Fprintf(os.Stdout, "%s : %s\n", f, art.Headline)
}

func findWarcFiles(start string) []string {
	files := []string{}
	err := filepath.Walk(start, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".warc" {
			return nil
		}

		files = append(files, path)
		return nil
	})

	if err != nil {
		panic(err)
	}
	return files
}

var opts struct {
	databaseURL string
}

func openStore(connStr string) (store.Store, error) {
	if connStr == "" {
		connStr = os.Getenv("SCRAPEOMAT_DB")
	}

	if connStr == "" {
		return nil, fmt.Errorf("no database specified (use -db flag or set $SCRAPEOMAT_DB)")
	}

	db, err := store.NewPgStore(connStr)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func main() {

	flag.StringVar(&opts.databaseURL, "db", "", "database connection string (eg postgres://scrapeomat:password@localhost/scrapeomat)")
	flag.Parse()

	db, err := openStore(opts.databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
	defer db.Close()

	var wg sync.WaitGroup

	runtime.GOMAXPROCS(runtime.NumCPU())
	fmt.Printf("MAXPROCS=%d dir=%s\n", runtime.GOMAXPROCS(0), flag.Arg(0))

	//	files := findWarcFiles(flag.Arg(0))

	files := flag.Args()

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
	in, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer in.Close()

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
