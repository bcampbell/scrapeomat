package store

// TODO: KILLKILLKILL!
type FetchedArt struct {
	Art *Article
	Err error
}

type Logger interface {
	Printf(format string, v ...interface{})
}
type nullLogger struct{}

func (l nullLogger) Printf(format string, v ...interface{}) {
}

type Store interface {
	Close()
	Stash(art *Article) (int, error)
	WhichAreNew(artURLs []string) ([]string, error)
	FindURLs(urls []string) ([]int, error)
	FetchCount(filt *Filter) (int, error)
	// TODO: fetch should return a cursor/iterator
	Fetch(filt *Filter) (<-chan FetchedArt, chan<- struct{})
	FetchPublications() ([]Publication, error)
	FetchSummary(filt *Filter, group string) ([]DatePubCount, error)
	FetchArt(artID int) (*Article, error)
}
