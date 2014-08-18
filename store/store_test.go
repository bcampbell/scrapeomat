package store

import (
	"github.com/bcampbell/arts/arts"
	"labix.org/v2/mgo/bson"
	"testing"
)

// Slightly evil testing using a live mongodb instance with a scratch database
func TestStash(t *testing.T) {

	testStore := NewMongoStore("localhost/test_scrapeomat")
	defer func() {
		testStore.session.DB("").DropDatabase()
		testStore.Close()
	}()

	// two versions of the same article
	art1 := &arts.Article{URLs: []string{"http://example.com/crappy-non-canonical-url/art1", "http://example.com/crufty-crap"}}

	art1b := &arts.Article{URLs: []string{"http://example.com/crappy-non-canonical-url/art1", "http://example.com/art1"}}

	// an unrelated article
	art2 := &arts.Article{URLs: []string{"http://example.com/art2"}}

	id1, err := testStore.Stash(art1)
	if err != nil {
		t.Errorf("Stash failed: %s", err)
		return
	}

	id1b, err := testStore.Stash(art1b)
	if err != nil {
		t.Errorf("stash failed to update: %s", err)
		return
	}
	n, err := testStore.session.DB("").C(testStore.artColl).FindId(bson.ObjectIdHex(id1)).Count()
	if err != nil || n != 0 {
		t.Error("Failed to remove old article")
	}

	// fetch and check the new article
	art1c := storedArticle{}
	err = testStore.session.DB("").C(testStore.artColl).FindId(bson.ObjectIdHex(id1b)).One(&art1c)
	if err != nil {
		t.Errorf("query failed: %s", err)
		return
	}
	// make sure we've got all 3 urls now
	if len(art1c.URLs) != 3 {
		t.Error("URL merging failed")
		return
	}

	// make sure adding an unlreated article works as planned
	_, err = testStore.Stash(art2)
	if err != nil {
		t.Errorf("Stash() failed: %s", err)
		return
	}

	totalArts, err := testStore.session.DB("").C(testStore.artColl).Count()
	if err != nil {
		t.Errorf("query failed: %s", err)
		return
	}
	expectedArts := 2
	if totalArts != expectedArts {
		t.Errorf("ended up with %d articles in store (expected %d)", totalArts, expectedArts)
		return
	}
}
