package sqlstore

import (
	//	"database/sql"
	"fmt"
)

func (ss *SQLStore) checkSchema() error {

	ver, err := ss.schemaVersion()
	if err != nil {
		return err
	}
	if ver == 7 {
		return nil // up to date.
	}

	// auto schema management currently only for sqlite.
	if ss.driverName != "sqlite3" {
		return fmt.Errorf("Missing Schema.")
	}

	if ver != 0 {
		return fmt.Errorf("No Schema upgrade path (from ver %d)", ver)
	}

	// TODO: handle schema upgrades for data-in-the-wild!

	stmts := []string{
		`CREATE TABLE publication (
			id INTEGER PRIMARY KEY,
			code TEXT NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			domain TEXT NOT NULL DEFAULT '')`,

		`CREATE TABLE article (
	        id INTEGER PRIMARY KEY,
			added TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		    canonical_url TEXT NOT NULL,
			headline TEXT NOT NULL,
	        content TEXT NOT NULL DEFAULT '',
		    published TIMESTAMP DEFAULT NULL,
			updated TIMESTAMP DEFAULT NULL,
	        publication_id INTEGER NOT NULL,
	        section TEXT NOT NULL DEFAULT '',
	        extra TEXT NOT NULL DEFAULT '',
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
		_, err := ss.db.Exec(stmt)
		if err != nil {
			return err
		}

	}

	return nil
}

func (ss *SQLStore) schemaVersion() (int, error) {
	var v int
	err := ss.db.QueryRow(`SELECT MAX(ver) FROM version`).Scan(&v)
	if err != nil {
		// should distinguish between missing version table and other errors,
		// but hey.
		return 0, nil
	}
	return v, nil
}
