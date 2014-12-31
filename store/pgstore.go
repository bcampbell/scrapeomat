package store

import (
	"database/sql"
	"fmt"
	"github.com/lib/pq"
	"regexp"
	"strings"
	"time"
)

// PgStore stashes articles in a postgresql db
type PgStore struct {
	db  *sql.DB
	loc *time.Location
}

// eg "postgres://username@localhost/dbname"
func NewPgStore(connStr string) (*PgStore, error) {

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	// our assumed location for publication dates, when no timezone given
	// TODO: this is the wrong place for it. Scraper should handle this on a per-publication basis
	loc, err := time.LoadLocation("Europe/London")
	if err != nil {
		return nil, err
	}

	store := PgStore{db: db, loc: loc}

	return &store, nil
}

func (store *PgStore) Close() {
	if store.db != nil {
		store.db.Close()
		store.db = nil
	}
}

func (store *PgStore) Stash(art *Article) (string, error) {
	tx, err := store.db.Begin()
	if err != nil {
		return "", err
	}
	artID, err := store.stash2(tx, art)
	if err != nil {
		tx.Rollback()
		return "", err
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

func (store *PgStore) cvtTime(timestamp string) pq.NullTime {
	for _, layout := range timeFmts {
		t, err := time.ParseInLocation(layout, timestamp, store.loc)
		if err == nil {
			return pq.NullTime{Time: t, Valid: true}
		}
	}

	return pq.NullTime{Valid: false}
}

var datePat = regexp.MustCompile(`^\d\d\d\d-\d\d-\d\d`)

func (store *PgStore) stash2(tx *sql.Tx, art *Article) (string, error) {

	pubID, err := store.findOrCreatePublication(tx, &art.Publication)
	if err != nil {
		return "", err
	}

	var artID int
	err = tx.QueryRow(`INSERT INTO article(canonical_url, headline, content, published, updated, publication_id) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`,
		art.CanonicalURL,
		art.Headline,
		art.Content,
		store.cvtTime(art.Published),
		store.cvtTime(art.Updated),
		pubID).Scan(&artID)
	if err != nil {
		return "", err
	}

	for _, u := range art.URLs {
		_, err = tx.Exec(`INSERT INTO article_url(article_id,url) VALUES($1,$2)`, artID, u)
		if err != nil {
			return "", fmt.Errorf("failed adding url %s: %s", u, err)
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
			return "", err
		}
		_, err = tx.Exec(`INSERT INTO author_attr(author_id,article_id) VALUES ($1, $2)`,
			authorID,
			artID)
		if err != nil {
			return "", err
		}
	}

	return "", nil
}

func (store *PgStore) WhichAreNew(artURLs []string) ([]string, error) {

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

func (store *PgStore) findOrCreatePublication(tx *sql.Tx, pub *Publication) (int, error) {
	var pubID int
	var err error

	if pub.Code == "" {
		return 0, fmt.Errorf("No publication code")
	}

	err = tx.QueryRow(`SELECT id FROM publication WHERE code=$1`, pub.Code).Scan(&pubID)
	if err == nil {
		return pubID, nil // return existing publication
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	// create new
	err = tx.QueryRow(`INSERT INTO publication(code,name,domain) VALUES($1,$2,$3) RETURNING id`,
		pub.Code,
		pub.Name,
		pub.Domain).Scan(&pubID)
	if err != nil {
		return 0, err
	}
	return pubID, nil
}

type frag struct {
	fmt    string
	params []interface{}
}

func (f frag) build(baseIdx int) (string, []interface{}) {
	indices := make([]interface{}, len(f.params))
	for i := 0; i < len(f.params); i++ {
		indices[i] = baseIdx + i
	}
	return fmt.Sprintf(f.fmt, indices...), f.params
}

type fragList []frag

func (l *fragList) Add(fmt string, params ...interface{}) {
	*l = append(*l, frag{fmt, params})
}
func (frags *fragList) Render() (string, []interface{}) {

	var idx int = 1
	params := []interface{}{}
	subStrs := []string{}
	for _, f := range *frags {
		s, p := f.build(idx)
		subStrs = append(subStrs, s)
		params = append(params, f.params...)
		idx += len(p)
	}

	return strings.Join(subStrs, " AND "), params
}

func buildWhere(filt *Filter) *fragList {
	//var idx int = 1
	frags := &fragList{}

	if !filt.PubFrom.IsZero() {
		frags.Add("a.published>=$%d", filt.PubFrom)
	}
	if !filt.PubTo.IsZero() {
		frags.Add("a.published<$%d", filt.PubTo)
	}
	if !filt.AddedFrom.IsZero() {
		frags.Add("a.added>=$%d", filt.AddedFrom)
	}
	if !filt.AddedTo.IsZero() {
		frags.Add("a.added<$%d", filt.AddedTo)
	}

	if len(filt.PubCodes) > 0 {
		foo := []string{}
		bar := []interface{}{}
		for _, code := range filt.PubCodes {
			foo = append(foo, "$%d")
			bar = append(bar, code)
		}
		frags.Add("p.code IN ("+strings.Join(foo, ",")+")", bar...)
	}

	return frags
}

func (store *PgStore) FetchCount(filt *Filter) (int, error) {
	whereClause, params := buildWhere(filt).Render()
	q := `SELECT COUNT(*)
           FROM (article a INNER JOIN publication p ON a.publication_id=p.id)
           WHERE ` + whereClause
	var cnt int
	err := store.db.QueryRow(q, params...).Scan(&cnt)
	return cnt, err
}

func (store *PgStore) Fetch(abort <-chan struct{}, filt *Filter) <-chan FetchedArt {

	whereClause, params := buildWhere(filt).Render()
	c := make(chan FetchedArt)
	go func() {
		defer close(c)

		q := `SELECT a.id,a.headline,a.canonical_url,a.content,a.published,a.updated,p.code,p.name,p.domain
	               FROM (article a INNER JOIN publication p ON a.publication_id=p.id)
	               WHERE ` + whereClause + ` ORDER BY published DESC`

		if filt.Limit > 0 {
			q += fmt.Sprintf(" LIMIT %d", filt.Limit)
		}
		if filt.Offset > 0 {
			q += fmt.Sprintf(" OFFSET %d", filt.Offset)
		}

		artRows, err := store.db.Query(q, params...)
		if err != nil {
			c <- FetchedArt{nil, err}
			return
		}
		defer artRows.Close()
		for artRows.Next() {
			select {
			case <-abort:
				fmt.Printf("fetch aborted.\n")
				return
			default:
			}

			var id int
			var art Article
			var p = &art.Publication

			var published, updated pq.NullTime
			if err := artRows.Scan(&id, &art.Headline, &art.CanonicalURL, &art.Content, &published, &updated, &p.Code, &p.Name, &p.Domain); err != nil {
				c <- FetchedArt{nil, err}
				return
			}

			if published.Valid {
				art.Published = published.Time.Format(time.RFC3339)
			}
			if updated.Valid {
				art.Updated = updated.Time.Format(time.RFC3339)
			}

			urls, err := store.fetchURLs(id)
			if err != nil {
				c <- FetchedArt{nil, err}
				return
			}
			art.URLs = urls

			// TODO: authors, keywords

			//			fmt.Printf("send %d: %s\n", id, art.Headline)
			c <- FetchedArt{&art, nil}

		}
		if err := artRows.Err(); err != nil {
			c <- FetchedArt{nil, err}
			return
		}

	}()
	return c
}

func (store *PgStore) fetchURLs(artID int) ([]string, error) {
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
