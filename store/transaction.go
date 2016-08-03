package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

type Transaction struct {
	s   *Store
	tx  *sql.Tx
	err error
}

// Transaction public interface
/*
type Txer interface {
	Close() errpr
	Stash(art *Article) int
	Err() error
}*/
func newTransaction(store *Store) *Transaction {
	tx, err := store.db.Begin()
	if err != nil {
		// transaction is borked, but user can keep calling it, and
		// the error will be returned upon Close()
		return &Transaction{s: store, tx: nil, err: err}
	} else {
		return &Transaction{s: store, tx: tx, err: err}
	}
}

func (t *Transaction) Close() error {

	if t.tx == nil {
		return t.err
	}

	var e2 error
	if t.err == nil {
		e2 = t.tx.Commit()
	} else {
		e2 = t.tx.Rollback()
	}
	t.tx = nil // to handle double-close

	if e2 != nil {
		t.err = e2
	}

	return t.err
}

func (t *Transaction) Stash(art *Article) int {
	if t.err != nil {
		return 0
	}
	tx := t.tx
	pubID, err := t.findOrCreatePublication(&art.Publication)
	if err != nil {
		t.err = err
		return 0
	}

	artID := art.ID

	extra := []byte{}
	if art.Extra != nil {
		extra, err = json.Marshal(art.Extra)
		if err != nil {
			t.err = err
			return 0
		}
	}

	if artID == 0 {
		// it's a new article
		err = tx.QueryRow(`INSERT INTO article(canonical_url, headline, content, published, updated, publication_id, section,extra) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
			art.CanonicalURL,
			art.Headline,
			art.Content,
			t.s.cvtTime(art.Published),
			t.s.cvtTime(art.Updated),
			pubID,
			art.Section,
			extra).Scan(&artID)
		if err != nil {
			t.err = err
			return 0
		}
	} else {
		// updating an existing article

		_, err = tx.Exec(`UPDATE article SET (canonical_url, headline, content, published, updated, publication_id, section,extra,added) = ($1,$2,$3,$4,$5,$6,$7,$8,NOW()) WHERE id=$9`,
			art.CanonicalURL,
			art.Headline,
			art.Content,
			t.s.cvtTime(art.Published),
			t.s.cvtTime(art.Updated),
			pubID,
			art.Section,
			extra,
			artID)
		if err != nil {
			t.err = err
			return 0
		}

		// delete old urls
		_, err = tx.Exec(`DELETE FROM article_url WHERE article_id=$1`, artID)
		if err != nil {
			t.err = err
			return 0
		}

		// delete old keywords
		_, err = tx.Exec(`DELETE FROM article_keyword WHERE article_id=$1`, artID)
		if err != nil {
			t.err = err
			return 0
		}

		// delete old authors
		_, err = tx.Exec(`DELETE FROM author WHERE id IN (SELECT author_id FROM author_attr WHERE article_id=$1)`, artID)
		if err != nil {
			t.err = err
			return 0
		}
		_, err = tx.Exec(`DELETE FROM author_attr WHERE article_id=$1`, artID)
		if err != nil {
			t.err = err
			return 0
		}
	}

	for _, u := range art.URLs {
		_, err = tx.Exec(`INSERT INTO article_url(article_id,url) VALUES($1,$2)`, artID, u)
		if err != nil {
			t.err = fmt.Errorf("failed adding url %s: %s", u, err)
			return 0
		}
	}

	for _, k := range art.Keywords {
		_, err = tx.Exec(`INSERT INTO article_keyword(article_id,name,url) VALUES($1,$2,$3)`,
			artID,
			k.Name,
			k.URL)
		if err != nil {
			t.err = fmt.Errorf("failed adding keyword %s (%s): %s", k.Name, k.URL, err)
			return 0
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
			t.err = err
			return 0
		}
		_, err = tx.Exec(`INSERT INTO author_attr(author_id,article_id) VALUES ($1, $2)`,
			authorID,
			artID)
		if err != nil {
			t.err = err
			return 0
		}
	}

	// all good.
	return artID
}

func (t *Transaction) findOrCreatePublication(pub *Publication) (int, error) {
	pubID, err := t.findPublication(pub)
	if err != nil {
		return 0, err
	}
	if pubID != 0 {
		return pubID, nil
	}
	return t.createPublication(pub)
}

// returns 0 if no match
func (t *Transaction) findPublication(pub *Publication) (int, error) {
	var pubID int
	var err error

	if pub.Code != "" {

		err = t.tx.QueryRow(`SELECT id FROM publication WHERE code=$1`, pub.Code).Scan(&pubID)
		if err == nil {
			return pubID, nil // return existing publication
		}
		if err != sql.ErrNoRows {
			return 0, err
		}
	}

	if pub.Name != "" {

		err = t.tx.QueryRow(`SELECT id FROM publication WHERE name=$1`, pub.Name).Scan(&pubID)
		if err == nil {
			return pubID, nil // return existing publication
		}
		if err != sql.ErrNoRows {
			return 0, err
		}
	}

	// TODO: publications can have multiple domains...
	if pub.Domain != "" {
		err = t.tx.QueryRow(`SELECT id FROM publication WHERE domain=$1`, pub.Domain).Scan(&pubID)
		if err == nil {
			return pubID, nil // return existing publication
		}
		if err != sql.ErrNoRows {
			return 0, err
		}
	}

	return 0, nil // no match
}

func (t *Transaction) createPublication(pub *Publication) (int, error) {
	// create new
	var pubID int
	err := t.tx.QueryRow(`INSERT INTO publication(code,name,domain) VALUES($1,$2,$3) RETURNING id`,
		pub.Code,
		pub.Name,
		pub.Domain).Scan(&pubID)
	if err != nil {
		return 0, err
	}
	return pubID, nil
}
