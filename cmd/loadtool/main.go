package main

import (
	"encoding/json"
	"flag"
	"fmt"
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
	databaseURL string
	pubCode     string
}

func main() {

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: loadtool [options] <dir-containing-json-articles>\n")
		flag.PrintDefaults()
		os.Exit(2)
	}

	flag.StringVar(&opts.databaseURL, "db", "", "database connection `string` (eg postgres://scrapeomat:password@localhost/scrapeomat)")
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

	for _, f := range jsonFiles {
		_, err = loadArt(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

}

type Art struct {
	URL       string `json:"url,omitempty"`
	Headline  string `json:"headline,omitempty"`
	Byline    string `json:"byline,omitempty"`
	Content   string `json:"content,omitempty"`
	Published string `json:"published,omitempty"`
	//	Publication Publication `json:"publication,omitempty"`
	Section string `json:"section,omitempty"`
}

func loadArt(filename string) (*store.Article, error) {

	fmt.Fprintf(os.Stderr, "load %s\n", filename)
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

	fmt.Printf("%+v\n", ConvertArticle(&a))

	return nil, nil
}

func ConvertArticle(src *Art) *store.Article {
	art := &store.Article{
		CanonicalURL: src.URL,
		URLs:         []string{},
		Headline:     src.Headline,
		Authors:      []store.Author{},
		Content:      src.Content,
		Published:    src.Published,
		//		Updated:      src.Updated,
		Publication: store.Publication{},
		Keywords:    []store.Keyword{},
		Section:     src.Section,
	}

	art.URLs = append(art.URLs, src.URL)
	art.Authors = append(art.Authors, store.Author{Name: src.Byline})
	art.Publication.Code = opts.pubCode

	return art
}
