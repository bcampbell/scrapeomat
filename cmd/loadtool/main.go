package main

// load dumped articles into scrapeomat db.
// work in progress - fix as required ;-)

import (
	"flag"
	"fmt"
	"os"
	"strings"
	//"time"
	"path/filepath"

	"github.com/bcampbell/scrapeomat/store"
	"github.com/bcampbell/scrapeomat/store/sqlstore"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

type Art struct {
	store.Article
	// some convenience fields
	URL     string `json:"url,omitempty"`
	Byline  string `json:"byline,omitempty"`
	Pubcode string `json:"pubcode,omitempty"`
}

// article stream from a slurp API has each article in own object:
// {article: {...}}
// {article: {...}}
type WireFmt struct {
	Art `json:"article,omitempty"`
}

var opts struct {
	driver           string
	connStr          string
	pubCode          string
	ignoreLoadErrors bool
	htmlEscape       bool
	recursive        bool
	forceUpdate      bool
}

const usageTxt = `usage: loadtool [options] [file(s)]>

Imports articles from json files into a scrapeomat db.
Input json format is same as slurp API output.
`

func main() {

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usageTxt)
		flag.PrintDefaults()
		os.Exit(2)
	}

	//	flag.BoolVar(&opts.ignoreLoadErrors, "i", false, "ignore load errors - skip failed art and continue")
	flag.BoolVar(&opts.recursive, "r", false, "Recursive - descend into dirs to find json files.")
	flag.StringVar(&opts.connStr, "db", "", "database connection string (or set SCRAPEOMAT_DB")
	flag.StringVar(&opts.driver, "driver", "", "database driver name (defaults to sqlite3 if SCRAPEOMAT_DRIVER is unset)")
	flag.BoolVar(&opts.forceUpdate, "f", false, "force update of articles already in db")
	flag.StringVar(&opts.pubCode, "pubcode", "", "publication shortcode (if not in article data)")
	flag.BoolVar(&opts.htmlEscape, "e", false, "HTML-escape plain text content field")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "ERROR: no input files\n")
		os.Exit(1)
	}

	jsonFiles, err := collectFiles(flag.Args(), opts.recursive)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}

	db, err := sqlstore.NewWithEnv(opts.driver, opts.connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR opening db: %s\n", err)
		os.Exit(1)
	}
	defer db.Close()

	imp := NewImporter(db)
	imp.UpdateExisting = opts.forceUpdate

	for _, jsonFile := range jsonFiles {
		err := imp.ImportJSONFile(jsonFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			os.Exit(1)
		}
	}

}

// get a list of input files from the commandline args
func collectFiles(args []string, recurse bool) ([]string, error) {
	found := []string{}
	for _, name := range args {
		inf, err := os.Stat(name)
		if err != nil {
			return nil, err
		}
		if inf.IsDir() {
			if !recurse {
				return nil, fmt.Errorf("%s is a directory (did you want -r?)", name)
			}
			foo, err := findJsonFilesRecursive(name)
			if err != nil {
				return nil, err
			}
			found = append(found, foo...)
		} else {
			found = append(found, name)
		}
	}
	return found, nil
}

// recursively grab list of all json files under rootDir dir
func findJsonFilesRecursive(rootDir string) ([]string, error) {
	files := []string{}
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
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
