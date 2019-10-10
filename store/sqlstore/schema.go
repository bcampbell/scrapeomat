package sqlstore

import (
	"database/sql"
	"fmt"
)

func checkSchema(db *sql.DB) error {

	ver, err := schemaVersion(db)
	if err != nil {
		return err
	}
	if ver == 7 {
		return nil // up to date.
	}
	if ver != 0 {
		return fmt.Errorf("No upgrade path (from ver %d)", ver)
	}

	// TODO: handle schema upgrades

	stmts := []string{
		`CREATE TABLE publication (
			id INTEGER PRIMARY KEY,
			code TEXT NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			domain TEXT NOT NULL DEFAULT '')`,

		`CREATE TABLE article (
	        id INTEGER PRIMARY KEY,
		    canonical_url TEXT NOT NULL,
			headline TEXT NOT NULL,
	        content TEXT NOT NULL,
		    published TIMESTAMP NOT NULL,
			updated TIMESTAMP NOT NULL,
	        publication_id INTEGER NOT NULL,
	        section TEXT NOT NULL DEFAULT '',
	        extra TEXT NOT NULL DEFAULT '',
			added TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
			FOREIGN KEY(publication_id) REFERENCES publication(id) )`,

		`CREATE TABLE author (
		    id INTEGER PRIMARY KEY,
		    name TEXT NOT NULL,
		    rel_link TEXT NOT NULL DEFAULT '',
		    email TEXT NOT NULL DEFAULT '',
		    twitter TEXT NOT NULL DEFAULT '' )`,

		`CREATE TABLE author_attr (
		    id INTEGER PRIMARY KEY,
		    author_id INT NOT NULL,
		    article_id INT NOT NULL,
			FOREIGN KEY(author_id) REFERENCES author(id) ON DELETE CASCADE,
			FOREIGN KEY(article_id) REFERENCES article(id) ON DELETE CASCADE )`,
		`CREATE INDEX author_attr_artid ON author_attr(article_id)`,
		`CREATE INDEX author_attr_authorid ON author_attr(author_id)`,

		`CREATE TABLE article_tag (
			id INTEGER PRIMARY KEY,
			article_id INTEGER NOT NULL,
			tag TEXT NOT NULL,
			FOREIGN KEY(article_id) REFERENCES article(id) ON DELETE CASCADE )`,
		`CREATE INDEX article_tag_artid ON article_tag(article_id)`,

		`CREATE TABLE article_url (
			id INTEGER PRIMARY KEY,
			article_id INTEGER NOT NULL,
			url TEXT NOT NULL,
			FOREIGN KEY(article_id) REFERENCES article(id) ON DELETE CASCADE )`,
		`CREATE INDEX article_url_artid ON article_url(article_id)`,
		`CREATE INDEX article_url_url ON article_url(url)`,

		`CREATE TABLE article_keyword (
			id INTEGER PRIMARY KEY,
			article_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			FOREIGN KEY(article_id) REFERENCES article(id) ON DELETE CASCADE )`,
		`CREATE INDEX article_keyword_artid ON article_keyword(article_id)`,

		`CREATE TABLE version (ver INTEGER NOT NULL)`,
		`CREATE TABLE settings (n TEXT, v TEXT NOT NULL)`,

		`INSERT INTO version (ver) VALUES (7)`,
	}

	for _, stmt := range stmts {
		_, err := db.Exec(stmt)
		if err != nil {
			return err
		}

	}

	return nil
}

func schemaVersion(db *sql.DB) (int, error) {
	var n string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='article';`).Scan(&n)
	if err == sql.ErrNoRows {
		return 0, nil // no schema at all
	}
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='version';`).Scan(&n)
	if err == sql.ErrNoRows {
		return 1, nil // version 1: no version table :-)
	}
	if err != nil {
		return 0, err
	}

	var v int
	err = db.QueryRow(`SELECT MAX(ver) FROM version`).Scan(&v)
	if err != nil {
		return 0, err
	}

	return v, nil
}
