package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/lib/pq"
	"regexp"
	"strings"
	"time"
)

// SQLStore stashes articles in a postgresql db
type SQLStore struct {
	db       *sql.DB
	loc      *time.Location
	ErrLog   Logger
	DebugLog Logger
}

// eg "postgres://username@localhost/dbname"
func NewSQLStore(connStr string) (*SQLStore, error) {

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, err
	}

	// our assumed location for publication dates, when no timezone given
	// TODO: this is the wrong place for it. Scraper should handle this on a per-publication basis
	loc, err := time.LoadLocation("Europe/London")
	if err != nil {
		return nil, err
	}

	store := SQLStore{
		db:       db,
		loc:      loc,
		ErrLog:   nullLogger{}, // TODO: should log to stderr by default?
		DebugLog: nullLogger{},
	}

	return &store, nil
}

func (store *SQLStore) Close() {
	if store.db != nil {
		store.db.Close()
		store.db = nil
	}
}

func (store *SQLStore) Begin() *Transaction {
	return newTransaction(store)
}

func (store *SQLStore) Stash(art *Article) (int, error) {
	tx := store.Begin()
	artID := tx.Stash(art)
	err := tx.Close()
	return artID, err
}

var timeFmts = []string{
	time.RFC3339,
	"2006-01-02T15:04Z07:00",
	//	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02",
}

func (store *SQLStore) cvtTime(timestamp string) pq.NullTime {
	for _, layout := range timeFmts {
		t, err := time.ParseInLocation(layout, timestamp, store.loc)
		if err == nil {
			return pq.NullTime{Time: t, Valid: true}
		}
	}

	return pq.NullTime{Valid: false}
}

var datePat = regexp.MustCompile(`^\d\d\d\d-\d\d-\d\d`)

// returns 0,nil if not found
/*
TODO: handle multiple matches...
func (store *SQLStore) FindArticle(artURLs []string) (int, error) {

	frags := make(fragList, 0, len(artURLs))
	for _, u := range artURLs {
		frags.Add("?", u)
	}
	foo, params := frags.Render(1, ",")
	var artID int
	s := `SELECT DISTINCT article_id FROM article_url WHERE url IN (` + foo + `)`
	err := store.db.QueryRow(s, params...).Scan(&artID)
	if err == sql.ErrNoRows {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	return artID, nil
}
*/

// FindURLs Looks up article urls, returning a list of matching article IDs.
// usually you'd use this on the URLs for a single article, expecting zero or one IDs back,
// but there's no reason you can't look up a whole bunch of articles at once, although you won't
// know which ones match which URLs.
// remember that there can be multiple URLs for a single article, AND also multiple articles can
// share the same URL (hopefully much much more rare).
func (store *SQLStore) FindURLs(urls []string) ([]int, error) {

	params := make([]interface{}, len(urls))
	placeholders := make([]string, len(urls))
	for i, u := range urls {
		params[i] = u
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	s := `SELECT distinct article_id FROM article_url WHERE url IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := store.db.Query(s, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []int{}
	for rows.Next() {
		var artID int
		if err := rows.Scan(&artID); err != nil {
			return nil, err
		}

		out = append(out, artID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// NOTE: remember article urls don't _have_ to be unique. If you only pass
// canonical urls in here you should be ok :-)
func (store *SQLStore) WhichAreNew(artURLs []string) ([]string, error) {

	stmt, err := store.db.Prepare(`SELECT article_id FROM article_url WHERE url=$1`)
	if err != nil {
		return nil, err
	}

	newArts := []string{}
	for _, u := range artURLs {
		var artID int
		err = stmt.QueryRow(u).Scan(&artID)
		if err == sql.ErrNoRows {
			newArts = append(newArts, u)
		} else if err != nil {
			return nil, err
		}
	}
	return newArts, nil
}

func buildWhere(filt *Filter) *fragList {
	//var idx int = 1
	frags := &fragList{}

	if !filt.PubFrom.IsZero() {
		frags.Add("a.published>=?", filt.PubFrom)
	}
	if !filt.PubTo.IsZero() {
		frags.Add("a.published<?", filt.PubTo)
	}
	if !filt.AddedFrom.IsZero() {
		frags.Add("a.added>=?", filt.AddedFrom)
	}
	if !filt.AddedTo.IsZero() {
		frags.Add("a.added<?", filt.AddedTo)
	}
	if filt.SinceID > 0 {
		frags.Add("a.id>?", filt.SinceID)
	}

	if len(filt.PubCodes) > 0 {
		foo := []string{}
		bar := []interface{}{}
		for _, code := range filt.PubCodes {
			foo = append(foo, "?")
			bar = append(bar, code)
		}
		frags.Add("p.code IN ("+strings.Join(foo, ",")+")", bar...)
	}

	if len(filt.XPubCodes) > 0 {
		foo := []string{}
		bar := []interface{}{}
		for _, code := range filt.XPubCodes {
			foo = append(foo, "?")
			bar = append(bar, code)
		}
		frags.Add("p.code NOT IN ("+strings.Join(foo, ",")+")", bar...)
	}

	return frags
}

func (store *SQLStore) FetchCount(filt *Filter) (int, error) {
	whereClause, params := buildWhere(filt).Render(1, " AND ")
	if whereClause != "" {
		whereClause = "WHERE " + whereClause
	}
	q := `SELECT COUNT(*)
           FROM (article a INNER JOIN publication p ON a.publication_id=p.id)
           ` + whereClause
	var cnt int
	err := store.db.QueryRow(q, params...).Scan(&cnt)
	return cnt, err
}

func (store *SQLStore) Fetch(filt *Filter) (<-chan FetchedArt, chan<- struct{}) {

	whereClause, params := buildWhere(filt).Render(1, " AND ")
	if whereClause != "" {
		whereClause = "WHERE " + whereClause
	}

	c := make(chan FetchedArt)
	abort := make(chan struct{}, 1) // buffering required to avoid deadlock TODO: is there a more elegant solution?
	go func() {
		defer close(c)
		defer close(abort)

		q := `SELECT a.id,a.headline,a.canonical_url,a.content,a.published,a.updated,a.section,a.extra,p.code,p.name,p.domain
	               FROM (article a INNER JOIN publication p ON a.publication_id=p.id)
	               ` + whereClause + ` ORDER BY id`

		if filt.Count > 0 {
			q += fmt.Sprintf(" LIMIT %d", filt.Count)
		}

		store.DebugLog.Printf("fetch: %s\n", q)
		store.DebugLog.Printf("fetch params: %+v\n", params)
		artRows, err := store.db.Query(q, params...)
		if err != nil {
			c <- FetchedArt{nil, err}
			return
		}
		defer artRows.Close()
		for artRows.Next() {
			select {
			case <-abort:
				store.DebugLog.Printf("fetch aborted.\n")
				return
			default:
			}

			var art Article
			var p = &art.Publication

			var published, updated pq.NullTime
			var extra []byte
			if err := artRows.Scan(&art.ID, &art.Headline, &art.CanonicalURL, &art.Content, &published, &updated, &art.Section, &extra, &p.Code, &p.Name, &p.Domain); err != nil {
				c <- FetchedArt{nil, err}
				return
			}

			if published.Valid {
				art.Published = published.Time.Format(time.RFC3339)
			}
			if updated.Valid {
				art.Updated = updated.Time.Format(time.RFC3339)
			}

			urls, err := store.fetchURLs(art.ID)
			if err != nil {
				c <- FetchedArt{nil, err}
				return
			}
			art.URLs = urls

			keywords, err := store.fetchKeywords(art.ID)
			if err != nil {
				c <- FetchedArt{nil, err}
				return
			}
			art.Keywords = keywords

			authors, err := store.fetchAuthors(art.ID)
			if err != nil {
				c <- FetchedArt{nil, err}
				return
			}
			art.Authors = authors

			// decode extra data
			if len(extra) > 0 {
				err = json.Unmarshal(extra, &art.Extra)
				if err != nil {
					err = fmt.Errorf("error in 'Extra' (artid %d): %s", art.ID, err)
					c <- FetchedArt{nil, err}
					return
				}
			}

			// TODO: keywords

			//			fmt.Printf("send %d: %s\n", art.ID, art.Headline)
			c <- FetchedArt{&art, nil}

		}
		if err := artRows.Err(); err != nil {
			c <- FetchedArt{nil, err}
			return
		}

	}()
	return c, abort
}

func (store *SQLStore) fetchURLs(artID int) ([]string, error) {
	rows, err := store.db.Query(`SELECT url FROM article_url WHERE article_id=$1`, artID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (store *SQLStore) fetchAuthors(artID int) ([]Author, error) {
	q := `SELECT name,rel_link,email,twitter
        FROM (author a INNER JOIN author_attr attr ON attr.author_id=a.id)
        WHERE article_id=$1`
	rows, err := store.db.Query(q, artID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Author{}
	for rows.Next() {
		var a Author
		if err := rows.Scan(&a.Name, &a.RelLink, &a.Email, &a.Twitter); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (store *SQLStore) fetchKeywords(artID int) ([]Keyword, error) {
	q := `SELECT name,url
        FROM article_keyword
        WHERE article_id=$1`
	rows, err := store.db.Query(q, artID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Keyword{}
	for rows.Next() {
		var k Keyword
		if err := rows.Scan(&k.Name, &k.URL); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (store *SQLStore) FetchPublications() ([]Publication, error) {
	rows, err := store.db.Query(`SELECT code,name,domain FROM publication ORDER by code`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Publication{}
	for rows.Next() {
		var p Publication
		if err := rows.Scan(&p.Code, &p.Name, &p.Domain); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil

}

type DatePubCount struct {
	Date    pq.NullTime
	PubCode string
	Count   int
}

func (store *SQLStore) FetchSummary(filt *Filter, group string) ([]DatePubCount, error) {
	whereClause, params := buildWhere(filt).Render(1, " AND ")
	if whereClause != "" {
		whereClause = "WHERE " + whereClause
	}

	var dayField string
	switch group {
	case "published":
		dayField = "a.published"
	case "added":
		dayField = "a.Added"
	default:
		return nil, fmt.Errorf("Bad group field (%s)", group)
	}

	q := `SELECT CAST( ` + dayField + ` AS DATE) AS day, p.code, COUNT(*)
	    FROM (article a INNER JOIN publication p ON a.publication_id=p.id) ` +
		whereClause + ` GROUP BY day, p.code ORDER BY day ASC ,p.code ASC;`

	store.DebugLog.Printf("summary: %s\n", q)
	store.DebugLog.Printf("summary params: %+v\n", params)

	rows, err := store.db.Query(q, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DatePubCount{}
	for rows.Next() {
		foo := DatePubCount{}
		if err := rows.Scan(&foo.Date, &foo.PubCode, &foo.Count); err != nil {
			return nil, err
		}
		out = append(out, foo)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	store.DebugLog.Printf("summary out: %d\n", len(out))
	return out, nil

}

// Fetch a single article by ID
func (store *SQLStore) FetchArt(artID int) (*Article, error) {

	q := `SELECT a.id,a.headline,a.canonical_url,a.content,a.published,a.updated,a.section,a.extra,p.code,p.name,p.domain
	               FROM (article a INNER JOIN publication p ON a.publication_id=p.id)
	               WHERE a.id=$1`

	store.DebugLog.Printf("fetch: %s [%d]\n", q, artID)
	row := store.db.QueryRow(q, artID)

	/* TODO: split scanning/augmenting out into function, to share with Fetch() */
	var art Article
	var p = &art.Publication

	var published, updated pq.NullTime
	var extra []byte
	if err := row.Scan(&art.ID, &art.Headline, &art.CanonicalURL, &art.Content, &published, &updated, &art.Section, &extra, &p.Code, &p.Name, &p.Domain); err != nil {
		return nil, err
	}

	if published.Valid {
		art.Published = published.Time.Format(time.RFC3339)
	}
	if updated.Valid {
		art.Updated = updated.Time.Format(time.RFC3339)
	}

	urls, err := store.fetchURLs(art.ID)
	if err != nil {
		return nil, err
	}
	art.URLs = urls

	keywords, err := store.fetchKeywords(art.ID)
	if err != nil {
		return nil, err
	}
	art.Keywords = keywords

	authors, err := store.fetchAuthors(art.ID)
	if err != nil {
		return nil, err
	}
	art.Authors = authors

	// decode extra data
	if len(extra) > 0 {
		err = json.Unmarshal(extra, &art.Extra)
		if err != nil {
			err = fmt.Errorf("error in 'Extra' (artid %d): %s", art.ID, err)
			return nil, err
		}
	}

	/* end scanning/augmenting */

	return &art, nil
}
