package store

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
)

// PgStore stashes articles in a postgresql db
type PgStore struct {
	db *sql.DB
}

// eg "postgres://username@localhost/dbname"
func NewPgStore(connStr string) (*PgStore, error) {

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	store := PgStore{db: db}
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
		art.Published,
		art.Updated,
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