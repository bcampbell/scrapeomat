package sqlstore

import (
	"github.com/bcampbell/scrapeomat/store"
	_ "github.com/mattn/go-sqlite3"
	"testing"
)

func TestStuff(t *testing.T) {

	ss, err := NewSQLStore("sqlite3", "file:/tmp/wibble.db")
	if err != nil {
		t.Errorf("NewSQLStore failed: %s\n", err)
		return
	}

	art := &store.Article{
		CanonicalURL: "http://example.com/foo-bar-wibble",
		Headline:     "Foo Bar Wibble",
		Content:      "<p>Foo, bar and Wibble.</p>",
		Published:    "2019-04-01",
		Updated:      "2019-04-01",
		Publication:  store.Publication{Code: "example"},
	}
	_, err = ss.Stash(art)
	if err != nil {
		t.Errorf("stash failed: %s\n", err)
		return
	}

	defer ss.Close()
}
