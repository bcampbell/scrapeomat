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
	Stash(art *Article) (int, error)
	WhichAreNew(artURLs []string) ([]string, error)
	FindURLs(urls []string) ([]int, error)
	FetchCount(filt *Filter) (int, error)
	Fetch(filt *Filter) ArtIter
	FetchPublications() ([]Publication, error)
	FetchSummary(filt *Filter, group string) ([]DatePubCount, error)
	FetchArt(artID int) (*Article, error)
}
