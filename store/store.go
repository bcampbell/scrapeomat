package store

import (
	"time"
)

type Logger interface {
	Printf(format string, v ...interface{})
}
type ArtIter interface {
	Next() bool
	Article() *Article
	Err() error
	Close() error
}

type DatePubCount struct {
	Date    time.Time
	PubCode string
	Count   int
}

type Store interface {
	Close()
	Stash(arts ...*Article) ([]int, error)
	WhichAreNew(artURLs []string) ([]string, error)
	FindURLs(urls []string) ([]int, error)
	FetchCount(filt *Filter) (int, error)
	Fetch(filt *Filter) ArtIter
	FetchPublications() ([]Publication, error)
	FetchSummary(filt *Filter, group string) ([]DatePubCount, error)
	FetchArt(artID int) (*Article, error)
}

// TODO:
// Need a cleaner definition of what's happening when we Stash articles.
// For example, there's currently no simple way to add additional URLs to
// an existing article.
//
// The common case we should optimise for:
// We have a bunch of scraped articles. We don't know if they are in the
// DB or not.
// If an article already in db, we should merge it with existing entry
// (at the very least, we should add any missing URLs).
// Otherwise add it as a new article.
// Maybe reject articles with an already-known ID?
// Have a separate Update/Replace fn for those?
//
// See cmd/loadtool FancyStash() for a speculative implementation...
//
