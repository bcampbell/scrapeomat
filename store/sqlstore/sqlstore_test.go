package sqlstore

import (
	"fmt"
	"github.com/bcampbell/scrapeomat/store"
	_ "github.com/mattn/go-sqlite3"
	"testing"
	"time"
)

func TestStuff(t *testing.T) {

	ss, err := New("sqlite3", "file:/tmp/wibble.db")
	if err != nil {
		t.Errorf("New: %s\n", err)
		return
	}

	doStash(t, ss)

	defer ss.Close()
}

func doStash(t *testing.T, ss *SQLStore) {

	testArts := []*store.Article{
		{
			CanonicalURL: "http://example.com/foo-bar-wibble",
			Headline:     "Foo Bar Wibble",
			Content:      "<p>Foo, bar and Wibble.</p>",
			Published:    "2019-04-01",
			Updated:      "2019-04-01",
			Publication:  store.Publication{Code: "example"},
		},
		{
			CanonicalURL: "http://example.com/blah-blah",
			Headline:     "Blah Blah",
			Content:      "<p>Blah blah blah. Blah.</p>",
			Published:    "2019-04-02",
			Updated:      "2019-04-02",
			Publication:  store.Publication{Code: "example"},
		},
	}

	//
	ids, err := ss.Stash(testArts...)
	if err != nil {
		t.Fatalf("stash failed: %s", err)
	}
	if len(ids) != len(testArts) {
		t.Fatalf("wrong article count (got %d, expected %d)",
			len(ids), len(testArts))
	}

	// check FetchCount()
	cnt, err := ss.FetchCount(&store.Filter{})
	if err != nil {
		t.Fatalf("FetchCount fail: %s", err)
	}
	if cnt != len(testArts) {
		t.Fatalf("FetchCount wrong (got %d, expected %d)",
			cnt, len(testArts))
	}

	// Now read them back
	lookup := map[string]*store.Article{}
	for _, art := range testArts {
		lookup[art.CanonicalURL] = art
	}

	it := ss.Fetch(&store.Filter{})
	fetchCnt := 0
	for it.Next() {
		got := it.Article()
		expect, ok := lookup[got.CanonicalURL]
		if !ok {
			t.Fatalf("Fetch returned unexpected article")
		}
		if got.Headline != expect.Headline {
			t.Fatalf("Fetch mismatch headline")
		}
		// TODO: check other fields here
		fetchCnt++
	}
	if fetchCnt != len(testArts) {
		t.Fatalf("Fetch count wrong (got %d, expected %d)",
			fetchCnt, len(testArts))
	}

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
