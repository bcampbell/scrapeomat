package store

import (
	//	"encoding/json"
	"fmt"
)

type FetchedArt struct {
	Art *Article
	Err error
}

type Store interface {
	WhichAreNew([]string) ([]string, error)

	// Stash saves an article to the store.
	// If the article is already in the store, the old one will be replaced
	// with the new one and the new ID returned. The URLs in the new article
	// will be the union of URLs from both versions - ie _all_ the known urls
	// for the article.
	Stash(*Article) (string, error)
	Close()

	Fetch(abort <-chan struct{}) (c <-chan FetchedArt)
}

// TestStore is a null store which does nothing
type TestStore struct{}

func NewTestStore() *TestStore {
	store := &TestStore{}
	return store
}
func (store *TestStore) Close() {
}

func (store *TestStore) WhichAreNew(artURLs []string) ([]string, error) {
	return artURLs, nil
}

func (store *TestStore) Stash(art *Article) (string, error) {
	u := art.CanonicalURL
	if u == "" {
		u = art.URLs[0]
	}
	fmt.Printf("%s \"%s\" [%s]\n", art.Published, art.Headline, art.CanonicalURL)
	return "", nil
}

func (store *TestStore) Fetch(abort <-chan struct{}) chan<- FetchedArt {

	c := make(chan FetchedArt)
	go func() {
		close(c)
	}()
	return c
}
