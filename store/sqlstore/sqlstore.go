package sqlstore

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/bcampbell/scrapeomat/store"
)

type nullLogger struct{}

func (l nullLogger) Printf(format string, v ...interface{}) {
}

type stderrLogger struct{}

func (l stderrLogger) Printf(format string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, format, v...)
}

// SQLStore stashes articles in an SQL database
type SQLStore struct {
	db         *sql.DB
	driverName string
	loc        *time.Location
	ErrLog     store.Logger
	DebugLog   store.Logger
}

type SQLArtIter struct {
	rows    *sql.Rows
	ss      *SQLStore
	current *store.Article
	err     error
}

// Which method to use to get last insert IDs
const (
	DUNNO     = iota
	RESULT    // use Result.LastInsertID()
	RETURNING // use sql "RETURNING" clause
)

// eg "postgres", "postgres://username@localhost/dbname"
// eg "sqlite3", "/tmp/foo.db"
func New(driver string, connStr string) (*SQLStore, error) {

	//db, err := sql.Open("postgres", connStr)
	db, err := sql.Open(driver, connStr)
	if err != nil {
		return nil, err
	}
	return NewFromDB(driver, db)
}

func NewFromDB(driver string, db *sql.DB) (*SQLStore, error) {
	err := db.Ping()
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

	ss := SQLStore{
		db:         db,
		driverName: driver,
		loc:        loc,
		ErrLog:     nullLogger{}, // TODO: should log to stderr by default?
		DebugLog:   nullLogger{},
	}

	// TODO: would be nice to have logger set up before here...
	err = ss.checkSchema()
	if err != nil {
		db.Close()
		return nil, err
	}

	return &ss, nil
}

// Same as New(), but if driver or connStr is missing, will try and read them
// from environment vars: SCRAPEOMAT_DRIVER & SCRAPEOMAT_DB.
// If both driver and SCRAPEOMAT_DRIVER are empty, default is "sqlite3".
func NewWithEnv(driver string, connStr string) (*SQLStore, error) {
	if connStr == "" {
		connStr = os.Getenv("SCRAPEOMAT_DB")
	}
	if driver == "" {
		driver = os.Getenv("SCRAPEOMAT_DRIVER")
		if driver == "" {
			driver = "sqlite3"
		}
	}

	if connStr == "" {
		return nil, fmt.Errorf("no database specified (set SCRAPEOMAT_DB?)")
	}

	return New(driver, connStr)
}

func (ss *SQLStore) Close() {
	if ss.db != nil {
		ss.db.Close()
		ss.db = nil
	}
}

func (ss *SQLStore) rebind(q string) string {
	return rebind(bindType(ss.driverName), q)
}

// can we use Result.LastInsertID() or do we need to fiddle the SQL?
func (ss *SQLStore) insertIDType() int {
	switch ss.driverName {
	case "postgres", "pgx", "pq-timeouts", "cloudsqlpostgres", "ql":
		return RETURNING
	case "sqlite3", "mysql":
		return RESULT
	case "oci8", "ora", "goracle":
		// ora: https://godoc.org/gopkg.in/rana/ora.v4#hdr-LastInsertId
		return DUNNO
	case "sqlserver":
		// https://github.com/denisenkom/go-mssqldb#important-notes
		return DUNNO
	default:
		return DUNNO
	}
}

// return a string with sql fn to return current timestamp.
func (ss *SQLStore) nowSQL() string {
	switch ss.driverName {
	case "postgres", "pgx", "pq-timeouts", "cloudsqlpostgres", "ql":
		return "NOW()"
	case "sqlite3":
		return "datetime('now','localtime')"
	case "mysql":
		return "PROPER_FN_GOES_HERE_PLEASE()"
	case "oci8", "ora", "goracle":
		return "PROPER_FN_GOES_HERE_PLEASE()"
	case "sqlserver":
		return "PROPER_FN_GOES_HERE_PLEASE()"
	default:
		return "PROPER_FN_GOES_HERE_PLEASE()"
	}
}

var timeFmts = []string{
	time.RFC3339,
	"2006-01-02T15:04Z07:00",
	//	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02",
}

func (ss *SQLStore) cvtTime(timestamp string) sql.NullTime {
	for _, layout := range timeFmts {
		t, err := time.ParseInLocation(layout, timestamp, ss.loc)
		if err == nil {
			return sql.NullTime{Time: t, Valid: true}
		}
	}

	return sql.NullTime{Valid: false}
}

var datePat = regexp.MustCompile(`^\d\d\d\d-\d\d-\d\d`)

// FindURLs Looks up article urls, returning a list of matching article IDs.
// usually you'd use this on the URLs for a single article, expecting zero or one IDs back,
// but there's no reason you can't look up a whole bunch of articles at once, although you won't
// know which ones match which URLs.
// remember that there can be multiple URLs for a single article, AND also multiple articles can
// share the same URL (hopefully much much more rare).
func (ss *SQLStore) FindURLs(urls []string) ([]int, error) {

	params := make([]interface{}, len(urls))
	placeholders := make([]string, len(urls))
	for i, u := range urls {
		params[i] = u
		placeholders[i] = "?"
	}

	s := `SELECT distinct article_id FROM article_url WHERE url IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := ss.db.Query(ss.rebind(s), params...)
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
func (ss *SQLStore) WhichAreNew(artURLs []string) ([]string, error) {

	stmt, err := ss.db.Prepare(ss.rebind(`SELECT article_id FROM article_url WHERE url=?`))
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

// Build a WHERE clause from a filter.
func buildWhere(filt *store.Filter) (string, []interface{}) {
	params := []interface{}{}
	frags := []string{}

	if !filt.PubFrom.IsZero() {
		frags = append(frags, "a.published>=?")
		params = append(params, filt.PubFrom)
	}
	if !filt.PubTo.IsZero() {
		frags = append(frags, "a.published<?")
		params = append(params, filt.PubTo)
	}
	if !filt.AddedFrom.IsZero() {
		frags = append(frags, "a.added>=?")
		params = append(params, filt.AddedFrom)
	}
	if !filt.AddedTo.IsZero() {
		frags = append(frags, "a.added<?")
		params = append(params, filt.AddedTo)
	}
	if filt.SinceID > 0 {
		frags = append(frags, "a.id>?")
		params = append(params, filt.SinceID)
	}

	if len(filt.PubCodes) > 0 {
		foo := []string{}
		bar := []interface{}{}
		for _, code := range filt.PubCodes {
			foo = append(foo, "?")
			bar = append(bar, code)
		}
		frags = append(frags, "p.code IN ("+strings.Join(foo, ",")+")")
		params = append(params, bar...)
	}

	if len(filt.XPubCodes) > 0 {
		foo := []string{}
		bar := []interface{}{}
		for _, code := range filt.XPubCodes {
			foo = append(foo, "?")
			bar = append(bar, code)
		}
		frags = append(frags, "p.code NOT IN ("+strings.Join(foo, ",")+")")
		params = append(params, bar...)
	}

	var whereClause string
	if len(frags) > 0 {
		whereClause = "WHERE " + strings.Join(frags, " AND ")
	}
	return whereClause, params
}

func (ss *SQLStore) FetchCount(filt *store.Filter) (int, error) {
	whereClause, params := buildWhere(filt)
	q := `SELECT COUNT(*)
           FROM (article a INNER JOIN publication p ON a.publication_id=p.id)
           ` + whereClause
	var cnt int
	err := ss.db.QueryRow(q, params...).Scan(&cnt)
	return cnt, err
}

func (ss *SQLStore) Fetch(filt *store.Filter) store.ArtIter {

	whereClause, params := buildWhere(filt)

	q := `SELECT a.id,a.headline,a.canonical_url,a.content,a.published,a.updated,a.section,a.extra,p.code,p.name,p.domain
	               FROM (article a INNER JOIN publication p ON a.publication_id=p.id)
	               ` + whereClause + ` ORDER BY a.id`

	if filt.Count > 0 {
		q += fmt.Sprintf(" LIMIT %d", filt.Count)
	}

	ss.DebugLog.Printf("fetch: %s\n", q)
	ss.DebugLog.Printf("fetch params: %+v\n", params)

	rows, err := ss.db.Query(ss.rebind(q), params...)
	return &SQLArtIter{ss: ss, rows: rows, err: err}
}

func (it *SQLArtIter) Close() error {
	// may not even have got as far as initing rows!
	var err error
	if it.rows != nil {
		err = it.rows.Close()
		it.rows = nil
	}
	return err
}

func (it *SQLArtIter) Err() error {
	return it.err
}

// if it returns true there will be an article.
func (it *SQLArtIter) Next() bool {
	it.current = nil
	if it.err != nil {
		return false // no more, if we're in error state
	}
	if !it.rows.Next() {
		it.err = it.rows.Err()
		return false // all done
	}

	art := &store.Article{}
	var p = &art.Publication

	var published, updated sql.NullTime
	var extra []byte
	err := it.rows.Scan(&art.ID, &art.Headline, &art.CanonicalURL, &art.Content, &published, &updated, &art.Section, &extra, &p.Code, &p.Name, &p.Domain)
	if err != nil {
		it.err = err
		return false
	}

	if published.Valid {
		art.Published = published.Time.Format(time.RFC3339)
	}
	if updated.Valid {
		art.Updated = updated.Time.Format(time.RFC3339)
	}

	urls, err := it.ss.fetchURLs(art.ID)
	if err != nil {
		it.err = err
		return false
	}
	art.URLs = urls

	keywords, err := it.ss.fetchKeywords(art.ID)
	if err != nil {
		it.err = err
		return false
	}
	art.Keywords = keywords

	authors, err := it.ss.fetchAuthors(art.ID)
	if err != nil {
		it.err = err
		return false
	}
	art.Authors = authors

	// decode extra data
	if len(extra) > 0 {
		err = json.Unmarshal(extra, &art.Extra)
		if err != nil {
			it.err = err
			return false
		}
	}

	// if we get this far there's an article ready.
	it.current = art
	return true
}

func (it *SQLArtIter) Article() *store.Article {
	return it.current
}

func (ss *SQLStore) fetchURLs(artID int) ([]string, error) {
	q := `SELECT url FROM article_url WHERE article_id=?`
	rows, err := ss.db.Query(ss.rebind(q), artID)
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

func (ss *SQLStore) fetchAuthors(artID int) ([]store.Author, error) {
	q := `SELECT name,rel_link,email,twitter
        FROM (author a INNER JOIN author_attr attr ON attr.author_id=a.id)
        WHERE article_id=?`
	rows, err := ss.db.Query(ss.rebind(q), artID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []store.Author{}
	for rows.Next() {
		var a store.Author
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

func (ss *SQLStore) fetchKeywords(artID int) ([]store.Keyword, error) {
	q := `SELECT name,url
        FROM article_keyword
        WHERE article_id=?`
	rows, err := ss.db.Query(ss.rebind(q), artID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []store.Keyword{}
	for rows.Next() {
		var k store.Keyword
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

func (ss *SQLStore) FetchPublications() ([]store.Publication, error) {
	q := `SELECT code,name,domain FROM publication ORDER by code`
	rows, err := ss.db.Query(ss.rebind(q))

	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []store.Publication{}
	for rows.Next() {
		var p store.Publication
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

func (ss *SQLStore) FetchSummary(filt *store.Filter, group string) ([]store.DatePubCount, error) {
	whereClause, params := buildWhere(filt)

	var dayField string
	switch group {
	case "published":
		dayField = "a.published"
	case "added":
		dayField = "a.Added"
	default:
		return nil, fmt.Errorf("Bad group field (%s)", group)
	}

	if ss.driverName != "sqlite3" {
		panic("TODO: postgresql")
		//q := `SELECT CAST( ` + dayField + ` AS DATE) AS day, p.code, COUNT(*)
		//    FROM (article a INNER JOIN publication p ON a.publication_id=p.id) ` +
		//	whereClause + ` GROUP BY day, p.code ORDER BY day ASC ,p.code ASC;`
	}

	// sqlite3
	q := `SELECT DATE(` + dayField + `) AS day, p.code, COUNT(*)
	    FROM (article a INNER JOIN publication p ON a.publication_id=p.id) ` +
		whereClause + ` GROUP BY day, p.code ORDER BY day ASC ,p.code ASC;`

	ss.DebugLog.Printf("summary: %s\n", q)
	ss.DebugLog.Printf("summary params: %+v\n", params)

	rows, err := ss.db.Query(ss.rebind(q), params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []store.DatePubCount{}
	for rows.Next() {
		foo := store.DatePubCount{}
		var day sql.NullString
		if err := rows.Scan(&day, &foo.PubCode, &foo.Count); err != nil {
			return nil, err
		}

		// TODO: sqlite3 driver can't seem to scan a DATE() to a time.Time (or sql.NullTime)
		// TODO: INVESTIGATE!
		// for now, workaround with string parsing.
		if day.Valid {
			t, err := time.Parse("2006-01-02", day.String)
			if err == nil {
				foo.Date = t
			}
		}
		//ss.DebugLog.Printf("summary: %v\n", foo)

		out = append(out, foo)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	ss.DebugLog.Printf("summary out: %d\n", len(out))
	return out, nil

}

// Fetch a single article by ID
func (ss *SQLStore) FetchArt(artID int) (*store.Article, error) {

	q := `SELECT a.id,a.headline,a.canonical_url,a.content,a.published,a.updated,a.section,a.extra,p.code,p.name,p.domain
	               FROM (article a INNER JOIN publication p ON a.publication_id=p.id)
	               WHERE a.id=?`

	ss.DebugLog.Printf("fetch: %s [%d]\n", q, artID)
	row := ss.db.QueryRow(q, artID)

	/* TODO: split scanning/augmenting out into function, to share with Fetch() */
	var art store.Article
	var p = &art.Publication

	var published, updated sql.NullTime
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

	urls, err := ss.fetchURLs(art.ID)
	if err != nil {
		return nil, err
	}
	art.URLs = urls

	keywords, err := ss.fetchKeywords(art.ID)
	if err != nil {
		return nil, err
	}
	art.Keywords = keywords

	authors, err := ss.fetchAuthors(art.ID)
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
