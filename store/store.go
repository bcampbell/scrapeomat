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

type Filter struct {
	// date ranges are [from,to)
	PubFrom   time.Time
	PubTo     time.Time
	AddedFrom time.Time
	AddedTo   time.Time
	// if empty, accept all publications (else only ones in list)
	PubCodes []string
	// exclude any publications in XPubCodes
	XPubCodes []string
	// Only return articles with ID > SinceID
	SinceID int
	// max number of articles wanted
	Count int
}

// Describe returns a concise description of the filter for logging/debugging/whatever
func (filt *Filter) Describe() string {
	s := "[ "

	if !filt.PubFrom.IsZero() && !filt.PubTo.IsZero() {
		s += fmt.Sprintf("pub %s..%s ", filt.PubFrom.Format(time.RFC3339), filt.PubTo.Format(time.RFC3339))
	} else if !filt.PubFrom.IsZero() {
		s += fmt.Sprintf("pub %s.. ", filt.PubFrom.Format(time.RFC3339))
	} else if !filt.PubTo.IsZero() {
		s += fmt.Sprintf("pub ..%s ", filt.PubTo.Format(time.RFC3339))
	}

	if !filt.AddedFrom.IsZero() && !filt.AddedTo.IsZero() {
		s += fmt.Sprintf("added %s..%s ", filt.AddedFrom.Format(time.RFC3339), filt.AddedTo.Format(time.RFC3339))
	} else if !filt.AddedFrom.IsZero() {
		s += fmt.Sprintf("added %s.. ", filt.AddedFrom.Format(time.RFC3339))
	} else if !filt.AddedTo.IsZero() {
		s += fmt.Sprintf("added ..%s ", filt.AddedTo.Format(time.RFC3339))
	}

	if len(filt.PubCodes) > 0 {
		s += strings.Join(filt.PubCodes, "|") + " "
	}

	if len(filt.XPubCodes) > 0 {
		foo := make([]string, len(filt.XPubCodes))
		for i, x := range filt.XPubCodes {
			foo[i] = "!" + x
		}
		s += strings.Join(foo, "|") + " "
	}

	if filt.Count > 0 {
		s += fmt.Sprintf("cnt %d ", filt.Count)
	}
	if filt.SinceID > 0 {
		s += fmt.Sprintf("since %d ", filt.SinceID)
	}

	s += "]"
	return s
}

// Store stashes articles in a postgresql db
type Store struct {
	db       *sql.DB
	loc      *time.Location
	ErrLog   Logger
	DebugLog Logger
}

// eg "postgres://username@localhost/dbname"
func NewStore(connStr string) (*Store, error) {

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

	store := Store{
		db:       db,
		loc:      loc,
		ErrLog:   nullLogger{}, // TODO: should log to stderr by default?
		DebugLog: nullLogger{},
	}

	return &store, nil
}

func (store *Store) Close() {
	if store.db != nil {
		store.db.Close()
		store.db = nil
	}
}

func (store *Store) Stash(art *Article) (int, error) {
	tx, err := store.db.Begin()
	if err != nil {
		return 0, err
	}
	artID, err := store.stash2(tx, art)
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	tx.Commit()
	return artID, nil
}

var timeFmts = []string{
	time.RFC3339,
	"2006-01-02T15:04Z07:00",
	//	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02",
}

func (store *Store) cvtTime(timestamp string) pq.NullTime {
	for _, layout := range timeFmts {
		t, err := time.ParseInLocation(layout, timestamp, store.loc)
		if err == nil {
			return pq.NullTime{Time: t, Valid: true}
		}
	}

	return pq.NullTime{Valid: false}
}

var datePat = regexp.MustCompile(`^\d\d\d\d-\d\d-\d\d`)

func (store *Store) stash2(tx *sql.Tx, art *Article) (int, error) {

	pubID, err := store.findOrCreatePublication(tx, &art.Publication)
	if err != nil {
		return 0, err
	}

	artID := art.ID

	extra := []byte{}
	if art.Extra != nil {
		extra, err = json.Marshal(art.Extra)
		if err != nil {
			return 0, err
		}
	}

	if artID == 0 {
		// it's a new article
		err = tx.QueryRow(`INSERT INTO article(canonical_url, headline, content, published, updated, publication_id, section,extra) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
			art.CanonicalURL,
			art.Headline,
			art.Content,
			store.cvtTime(art.Published),
			store.cvtTime(art.Updated),
			pubID,
			art.Section,
			extra).Scan(&artID)
		if err != nil {
			return 0, err
		}
	} else {
		// updating an existing article

		_, err = tx.Exec(`UPDATE article SET (canonical_url, headline, content, published, updated, publication_id, section,extra,added) = ($1,$2,$3,$4,$5,$6,$7,$8,NOW()) WHERE id=$9`,
			art.CanonicalURL,
			art.Headline,
			art.Content,
			store.cvtTime(art.Published),
			store.cvtTime(art.Updated),
			pubID,
			art.Section,
			extra,
			artID)
		if err != nil {
			return 0, err
		}

		// delete old urls (TODO: should merge?)
		_, err = tx.Exec(`DELETE FROM article_url WHERE article_id=$1`, artID)
		if err != nil {
			return 0, err
		}

		// delete old keywords (TODO: should merge?)
		_, err = tx.Exec(`DELETE FROM article_keyword WHERE article_id=$1`, artID)
		if err != nil {
			return 0, err
		}

		// delete old authors
		// TODO: either:
		//         1) get rid of author_attr
		//  or     2) make an attempt to resolve authors
		//  and/or 3) do a periodic sweep to zap orphaned authors
		_, err = tx.Exec(`DELETE FROM author WHERE id IN (SELECT author_id FROM author_attr WHERE article_id=$1)`, artID)
		if err != nil {
			return 0, err
		}
		_, err = tx.Exec(`DELETE FROM author_attr WHERE article_id=$1`, artID)
		if err != nil {
			return 0, err
		}
	}

	for _, u := range art.URLs {
		_, err = tx.Exec(`INSERT INTO article_url(article_id,url) VALUES($1,$2)`, artID, u)
		if err != nil {
			return 0, fmt.Errorf("failed adding url %s: %s", u, err)
		}
	}

	for _, k := range art.Keywords {
		_, err = tx.Exec(`INSERT INTO article_keyword(article_id,name,url) VALUES($1,$2,$3)`,
			artID,
			k.Name,
			k.URL)
		if err != nil {
			return 0, fmt.Errorf("failed adding keyword %s (%s): %s", k.Name, k.URL, err)
		}
	}

	for _, author := range art.Authors {
		var authorID int
		err = tx.QueryRow(`INSERT INTO author(name,rel_link,email,twitter) VALUES ($1,$2,$3,$4) RETURNING id`,
			author.Name,
			author.RelLink,
			author.Email,
			author.Twitter).Scan(&authorID)
		if err != nil {
			return 0, err
		}
		_, err = tx.Exec(`INSERT INTO author_attr(author_id,article_id) VALUES ($1, $2)`,
			authorID,
			artID)
		if err != nil {
			return 0, err
		}
	}

	// all good.
	return artID, nil
}

// returns 0,nil if not found
/*
TODO: handle multiple matches...
func (store *Store) FindArticle(artURLs []string) (int, error) {

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

// NOTE: remember article urls don't _have_ to be unique. If you only pass
// canonical urls in here you should be ok :-)
func (store *Store) WhichAreNew(artURLs []string) ([]string, error) {

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

func (store *Store) findOrCreatePublication(tx *sql.Tx, pub *Publication) (int, error) {
	pubID, err := store.findPublication(tx, pub)
	if err != nil {
		return 0, err
	}
	if pubID != 0 {
		return pubID, nil
	}
	return store.createPublication(tx, pub)
}

// returns 0 if no match
func (store *Store) findPublication(tx *sql.Tx, pub *Publication) (int, error) {
	var pubID int
	var err error

	if pub.Code != "" {

		err = tx.QueryRow(`SELECT id FROM publication WHERE code=$1`, pub.Code).Scan(&pubID)
		if err == nil {
			return pubID, nil // return existing publication
		}
		if err != sql.ErrNoRows {
			return 0, err
		}
	}

	if pub.Name != "" {

		err = tx.QueryRow(`SELECT id FROM publication WHERE name=$1`, pub.Name).Scan(&pubID)
		if err == nil {
			return pubID, nil // return existing publication
		}
		if err != sql.ErrNoRows {
			return 0, err
		}
	}

	// TODO: publications can have multiple domains...
	if pub.Domain != "" {
		err = tx.QueryRow(`SELECT id FROM publication WHERE domain=$1`, pub.Domain).Scan(&pubID)
		if err == nil {
			return pubID, nil // return existing publication
		}
		if err != sql.ErrNoRows {
			return 0, err
		}
	}

	return 0, nil // no match
}

func (store *Store) createPublication(tx *sql.Tx, pub *Publication) (int, error) {
	// create new
	var pubID int
	err := tx.QueryRow(`INSERT INTO publication(code,name,domain) VALUES($1,$2,$3) RETURNING id`,
		pub.Code,
		pub.Name,
		pub.Domain).Scan(&pubID)
	if err != nil {
		return 0, err
	}
	return pubID, nil
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

func (store *Store) FetchCount(filt *Filter) (int, error) {
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

func (store *Store) Fetch(filt *Filter) (<-chan FetchedArt, chan<- struct{}) {

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

func (store *Store) fetchURLs(artID int) ([]string, error) {
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

func (store *Store) fetchAuthors(artID int) ([]Author, error) {
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

func (store *Store) fetchKeywords(artID int) ([]Keyword, error) {
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

func (store *Store) FetchPublications() ([]Publication, error) {
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
