package sqlstore

import (
	"fmt"
	"github.com/bcampbell/scrapeomat/store"
	_ "github.com/mattn/go-sqlite3"
	"testing"
	"time"
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

func ExampleBuildWhere() {

	filt := &store.Filter{
		PubFrom:  time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC),
		PubTo:    time.Date(2010, 2, 1, 0, 0, 0, 0, time.UTC),
		PubCodes: []string{"dailynews", "dailyshoes"},
	}
	s, p := buildWhere(filt)

	fmt.Println(s)
	fmt.Println(rebind(bindType("sqllite3"), s))
	fmt.Println(rebind(bindType("postgres"), s))
	fmt.Println(p)
	// Output:
	// WHERE a.published>=? AND a.published<? AND p.code IN (?,?)
	// WHERE a.published>=? AND a.published<? AND p.code IN (?,?)
	// WHERE a.published>=$1 AND a.published<$2 AND p.code IN ($3,$4)
	// [2010-01-01 00:00:00 +0000 UTC 2010-02-01 00:00:00 +0000 UTC dailynews dailyshoes]
}
