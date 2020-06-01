package sqlstore

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/bcampbell/scrapeomat/store"
)

// TODO: break all these tests into individual functions.

func performDBTests(t *testing.T, ss *SQLStore) {

	testArts := []*store.Article{
		{
			CanonicalURL: "http://example.com/foo-bar-wibble",
			Headline:     "Foo Bar Wibble",
			Content:      "<p>Foo, bar and Wibble.</p>",
			Published:    "2019-04-01",
			Updated:      "2019-04-01",
			Publication:  store.Publication{Code: "example"},
			Authors: []store.Author{
				{Name: "Bob Smith"},
				{Name: "Fred Door"},
			},
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
	// the test articles now have IDs
	for idx, id := range ids {
		testArts[idx].ID = id
	}

	checkArticles(t, ss, testArts)

	// Update an article
	testArts[0].Headline = "A Revised Headline"
	ids, err = ss.Stash(testArts[0])
	if err != nil {
		t.Fatalf("stash failed: %s", err)
	}

	//
	checkArticles(t, ss, testArts)

	// Very basic FetchSummary() check
	mustDay := func(s string) time.Time {
		t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
		if err != nil {
			panic("bad time: " + s)
		}
		return t
	}
	expectedCounts := []store.DatePubCount{
		{mustDay("2019-04-01"), "example", 1},
		{mustDay("2019-04-02"), "example", 1},
	}
	counts, err := ss.FetchSummary(&store.Filter{}, "published")
	if !reflect.DeepEqual(expectedCounts, counts) {
		fmt.Printf("BUGGER.\n")
		t.Fatalf("FetchSummary: expected %v, got %v", expectedCounts, counts)
	}

	// Check FetchCount runs with a non-null filter
	cnt, err := ss.FetchCount(&store.Filter{
		PubFrom: mustDay("1821-01-01"),
		PubTo:   mustDay("2050-01-01"),
	})
	if err != nil {
		t.Fatalf("FetchCount fail: %s", err)
	}
	if cnt != len(testArts) {
		t.Fatalf("FetchCount wrong (got %d, expected %d)",
			cnt, len(testArts))
	}
}

func checkArticles(t *testing.T, ss *SQLStore, testArts []*store.Article) {
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
			t.Fatal("Fetch returned unexpected article")
		}
		if got.CanonicalURL != expect.CanonicalURL {
			t.Fatal("Fetch: mismatch canonical_url")
		}
		if !equalStrings(expect.URLs, got.URLs) {
			t.Fatalf("Fetch: mismatch urls (expect: %v got: %v)", expect.URLs, got.URLs)
		}
		if got.Headline != expect.Headline {
			t.Fatal("Fetch: mismatch headline")
		}

		//slice might be nil or [] empty...
		if len(expect.Authors) != 0 || len(got.Authors) != 0 {
			if !reflect.DeepEqual(expect.Authors, got.Authors) {
				t.Fatal("Fetch: mismatch authors")
			}
		}
		if got.Content != expect.Content {
			t.Fatal("Fetch: mismatch content")
		}
		if got.Section != expect.Section {
			t.Fatal("Fetch: mismatch section")
		}
		// TODO: check
		//  - Published, Updated fields here (requires parsing)
		//  - keywords
		fetchCnt++
	}
	if it.Err() != nil {
		t.Fatalf("Fetch failed: %s", it.Err())
	}
	if fetchCnt != len(testArts) {
		t.Fatalf("Fetch count wrong (got %d, expected %d)",
			fetchCnt, len(testArts))
	}
}

func equalStrings(a []string, b []string) bool {
	//slice might be nil or [] empty...
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
