package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/bcampbell/scrapeomat/store"
)

// Importer imports article data from JSON files into a scrapeomat store.
type Importer struct {
	DB             store.Store
	UpdateExisting bool // if true, update existing articles in db (else skip)

	arts []*store.Article // currently unflushed articles
}

const BATCHSIZE = 500

func NewImporter(db store.Store) *Importer {
	return &Importer{
		DB:             db,
		UpdateExisting: false,
		arts:           nil,
	}
}

func (imp *Importer) ImportJSONFile(jsonFile string) error {
	fp, err := os.Open(jsonFile)
	if err != nil {
		return err
	}
	defer fp.Close()

	fmt.Fprintf(os.Stderr, "%s\n", jsonFile)

	dec := json.NewDecoder(fp)

	// main article loop here
	for {
		var in Art
		err = dec.Decode(&in)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		art := convertArticle(&in)
		imp.arts = append(imp.arts, art)
		if len(imp.arts) >= BATCHSIZE {
			err = imp.flush()
			if err != nil {
				return err
			}
		}
	}

	return imp.flush()
}

func (imp *Importer) flush() error {
	if len(imp.arts) == 0 {
		return nil
	}
	err := FancyStash(imp.DB, imp.UpdateExisting, imp.arts...)
	if err != nil {
		return err
	}
	imp.arts = nil
	return nil
}

// try and catch stuff that'll screw up DB
func SanityCheckArticle(art *store.Article) error {
	if art.ID != 0 {
		return fmt.Errorf("Article already has ID (%d)", art.ID)
	}
	if art.CanonicalURL == "" && len(art.URLs) == 0 {
		return fmt.Errorf("Article has no URLs")
	}
	if art.Publication.Code == "" {
		return fmt.Errorf("Missing pubcode")
	}
	return nil
}

// Stash articles.
// This should be in core store interface?
func FancyStash(db store.Store, updateExisting bool, arts ...*store.Article) error {
	stashArts := []*store.Article{}
	updateArts := []*store.Article{} // contains subset of stashArts
	skipArts := []*store.Article{}
	badArts := []*store.Article{}

	for _, art := range arts {
		err := SanityCheckArticle(art)
		if err != nil {
			fmt.Fprintf(os.Stderr, "BAD: %s\n", err.Error())
			badArts = append(badArts, art)
			continue
		}
		// look it up in db
		urls := []string{}
		if art.CanonicalURL != "" {
			urls = append(urls, art.CanonicalURL)
		}
		urls = append(urls, art.URLs...)
		ids, err := db.FindURLs(urls)
		if len(ids) == 0 {
			// not in DB - it's new.
			stashArts = append(stashArts, art)
			continue
		}
		if len(ids) == 1 {
			// Already got this one.
			art.ID = ids[0]
			if updateExisting {
				// add to both stash and update lists
				stashArts = append(stashArts, art)
				updateArts = append(updateArts, art)
			} else {
				// skip it.
				skipArts = append(skipArts, art)
				continue
			}
		}
		if len(ids) > 1 {
			// Uhoh...
			fmt.Fprintf(os.Stderr, "BAD: multiple articles in DB for %q\n", urls)
			badArts = append(badArts, art)
			continue
		}
	}

	// stash the new articles
	_, err := db.Stash(stashArts...)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%d stashed (%d updated), %d skipped, %d bad\n", len(stashArts), len(updateArts), len(skipArts), len(badArts))

	return nil
}

func convertArticle(src *Art) *store.Article {
	out := store.Article(src.Article)

	// strip any existing ID
	out.ID = 0

	// if no 'canonical_url' or 'urls', try 'url'...
	if out.CanonicalURL == "" && len(out.URLs) == 0 && src.URL != "" {
		out.CanonicalURL = src.URL
	}

	// if no 'urls' use 'canonical_url'.
	if len(out.URLs) == 0 && out.CanonicalURL != "" {
		out.URLs = []string{out.CanonicalURL}
	}

	if opts.htmlEscape {
		out.Content = html.EscapeString(src.Content)
	}

	// TODO: handle byline better?
	if len(out.Authors) == 0 && src.Byline != "" {
		out.Authors = append(out.Authors, store.Author{Name: src.Byline})
	}

	// fill in pubcode if missing
	if out.Publication.Code == "" {
		if src.Pubcode != "" {
			out.Publication.Code = src.Pubcode
		} else if opts.pubCode != "" {
			out.Publication.Code = opts.pubCode
		} else {
			out.Publication.Code = pubCodeFromURL(out.CanonicalURL)
		}
	}
	return &out
}

func pubCodeFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	code := strings.ToLower(u.Hostname())
	code = strings.TrimPrefix(code, "www.")
	return code
}
