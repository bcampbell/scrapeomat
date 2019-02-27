package main

// load dumped articles into scrapeomat db.
// work in progress - fix as required ;-)

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/bcampbell/scrapeomat/store"
	"html"
	"os"
	"strings"
	//"time"
	"path/filepath"
)

// recursively grab list of all json files under start dir
func findJsonFiles(start string) ([]string, error) {
	files := []string{}
	err := filepath.Walk(start, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(path, ".json") {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

var opts struct {
	db               string
	pubCode          string
	ignoreLoadErrors bool
	htmlEscape       bool
}

func main() {

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: loadtool [options] <dir-containing-json-articles>\n")
		flag.PrintDefaults()
		os.Exit(2)
	}

	flag.BoolVar(&opts.ignoreLoadErrors, "i", false, "ignore load errors - skip failed art and continue")
	flag.BoolVar(&opts.htmlEscape, "e", false, "HTML-escape plain text content field")
	flag.StringVar(&opts.db, "db", "", "database connection `string` (eg postgres://scrapeomat:password@localhost/scrapeomat)")
	flag.StringVar(&opts.pubCode, "pubcode", "", "publication shortcode")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "ERROR: missing <dir-containing-json-articles>\n")
		os.Exit(1)
	}

	jsonFiles, err := findJsonFiles(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}

	/*
		// dump out for debugging
		for _, f := range jsonFiles {
			art, err := readArt(f)
			if err != nil {
				panic(err)
			}

			fmt.Printf("%+v\n", art)
		}
		return
	*/

	connStr := opts.db
	if connStr == "" {
		connStr = os.Getenv("SCRAPEOMAT_DB")
	}

	if connStr == "" {
		fmt.Fprintf(os.Stderr, "ERROR: no database specified (use -db flag or set $SCRAPEOMAT_DB)\n")
		os.Exit(1)
	}
	db, err := store.NewStore(connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR opening db: %s\n", err)
		os.Exit(1)
	}
	defer db.Close()

	batchsize := 500

	for i := 0; i < len(jsonFiles); i += batchsize {
		j := i + batchsize
		if j > len(jsonFiles) {
			j = len(jsonFiles)
		}
		err = loadBatch(db, jsonFiles[i:j])
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			os.Exit(1)
		}
	}
}

func loadBatch(db *store.Store, filenames []string) error {
	arts := map[string]*store.Article{}
	urls := []string{}
	for _, f := range filenames {
		art, err := readArt(f)
		if err != nil {
			if opts.ignoreLoadErrors {
				fmt.Fprintf(os.Stderr, "failed to load %s: %s\n", f, err)
				continue
			}
			return fmt.Errorf("%s: %s", f, err)
		}
		if art.Publication.Code == "" {
			return fmt.Errorf("Missing pubcode")
		}
		arts[art.CanonicalURL] = art
		urls = append(urls, art.CanonicalURL)
	}

	newOnes, err := db.WhichAreNew(urls)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "loaded %d arts (%d new)\n", len(arts), len(newOnes))

	for _, u := range newOnes {
		art := arts[u]
		_, err := db.Stash(art)
		if err != nil {
			return err
		}
	}
	return nil
}

type Art struct {
	store.Article
	// some convenience fields
	URL     string `json:"url,omitempty"`
	Byline  string `json:"byline,omitempty"`
	Pubcode string `json:"pubcode,omitempty"`
}

func readArt(filename string) (*store.Article, error) {

	//fmt.Fprintf(os.Stderr, "load %s\n", filename)
	var a Art

	fp, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	dec := json.NewDecoder(fp)
	err = dec.Decode(&a)
	if err != nil {
		return nil, err
	}

	out := ConvertArticle(&a)
	return out, nil
}

func ConvertArticle(src *Art) *store.Article {

	if src.URL != "" {
		src.CanonicalURL = src.URL
		src.URLs = []string{src.URL}
	}

	if opts.htmlEscape {
		src.Content = html.EscapeString(src.Content)
	}

	// TODO: handle byline better?
	if src.Byline != "" {
		src.Authors = append(src.Authors, store.Author{Name: src.Byline})
	}

	// fill in pubcode if missing
	if src.Publication.Code == "" {
		if src.Pubcode != "" {
			src.Publication.Code = src.Pubcode
		} else {
			src.Publication.Code = opts.pubCode
		}
	}
	return &src.Article
}
