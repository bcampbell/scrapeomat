package main

// load dumped articles into scrapeomat db.
// work in progress - fix as required ;-)

import (
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"os"
	"semprini/scrapeomat/store"
	"strings"
	//"time"
	"path/filepath"
)

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
	URL       string `json:"url,omitempty"`
	Headline  string `json:"headline,omitempty"`
	Byline    string `json:"byline,omitempty"`
	Content   string `json:"content,omitempty"`
	Published string `json:"published,omitempty"`
	Pubcode   string `json:"pubcode,omitempty"`
	//	Publication Publication `json:"publication,omitempty"`
	Section string `json:"section,omitempty"`
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

	//	fmt.Printf("%+v\n", ConvertArticle(&a))

	out := ConvertArticle(&a)
	return out, nil
}

func ConvertArticle(src *Art) *store.Article {

	art := &store.Article{
		CanonicalURL: src.URL,
		URLs:         []string{src.URL},
		Headline:     src.Headline,
		Authors:      []store.Author{},
		Content:      src.Content,
		Published:    src.Published,
		//		Updated:      src.Updated,
		Publication: store.Publication{},
		Keywords:    []store.Keyword{},
		Section:     src.Section,
	}

	if opts.htmlEscape {
		art.Content = html.EscapeString(art.Content)
	}

	if src.Byline != "" {
		art.Authors = append(art.Authors, store.Author{Name: src.Byline})
	}

	if src.Pubcode != "" {
		art.Publication.Code = src.Pubcode
	} else {
		art.Publication.Code = opts.pubCode
	}

	return art
}
